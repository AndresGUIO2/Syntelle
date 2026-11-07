package accounts

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"syntelle/api/internal/httpapi"
)

const maxPortraitBytes = 5_000_000
const characterColumns = `id,name,source_sha256,mime_type,width,height,status,coalesce(package_hash,''),coalesce(failure_code,''),created_at,updated_at,expires_at,source_bytes IS NOT NULL`
const expireCharacterSources = `UPDATE syntelle_accounts.character_creations SET source_bytes=NULL,
status=CASE WHEN status IN ('queued','processing') THEN 'failed' ELSE status END,
failure_code=CASE WHEN status IN ('queued','processing') THEN 'source_expired' ELSE failure_code END,
updated_at=$1 WHERE expires_at<=$1 AND source_bytes IS NOT NULL`

var characterID = regexp.MustCompile(`^char_[a-f0-9]{32}$`)
var creationRequestKey = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

type characterRecord struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	SourceSHA256      string    `json:"source_sha256"`
	MimeType          string    `json:"mime_type"`
	Width             int       `json:"width"`
	Height            int       `json:"height"`
	Status            string    `json:"status"`
	PackageHash       string    `json:"package_hash,omitempty"`
	FailureCode       string    `json:"failure_code,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	ExpiresAt         time.Time `json:"expires_at"`
	PortraitAvailable bool      `json:"portrait_available"`
}

func scanCharacter(row pgx.Row) (characterRecord, error) {
	var c characterRecord
	err := row.Scan(&c.ID, &c.Name, &c.SourceSHA256, &c.MimeType, &c.Width, &c.Height, &c.Status, &c.PackageHash, &c.FailureCode, &c.CreatedAt, &c.UpdatedAt, &c.ExpiresAt, &c.PortraitAvailable)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}

// ResolveCharacter is the private authority for choosing a custom render package.
func (s *Service) ResolveCharacter(ctx context.Context, accountID, id string) (string, error) {
	if accountID == "" || !characterID.MatchString(id) {
		return "", ErrNotFound
	}
	var hash string
	err := s.Pool.QueryRow(ctx, `SELECT package_hash FROM syntelle_accounts.character_creations
WHERE id=$1 AND account_id=$2 AND deleted_at IS NULL AND status='ready'`, id, accountID).Scan(&hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return hash, err
}

func (s *Service) characters(w http.ResponseWriter, r *http.Request) {
	p, ok := s.browserAccount(w, r)
	if !ok {
		return
	}
	if _, err := s.Pool.Exec(r.Context(), expireCharacterSources, s.cfg.Now()); err != nil {
		accessError(w, err)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/characters")
	if path == "" || path == "/" {
		switch r.Method {
		case http.MethodGet:
			rows, err := s.Pool.Query(r.Context(), "SELECT "+characterColumns+` FROM syntelle_accounts.character_creations WHERE account_id=$1 AND deleted_at IS NULL ORDER BY created_at DESC, id LIMIT 100`, p.AccountID)
			if err != nil {
				accessError(w, err)
				return
			}
			defer rows.Close()
			items := []characterRecord{}
			for rows.Next() {
				c, err := scanCharacter(rows)
				if err != nil {
					accessError(w, err)
					return
				}
				items = append(items, c)
			}
			if err := rows.Err(); err != nil {
				accessError(w, err)
				return
			}
			httpapi.WriteJSON(w, 200, map[string]any{"items": items})
		case http.MethodPost:
			s.createCharacter(w, r, p.AccountID)
		default:
			httpapi.WriteError(w, 405, "method_not_allowed", "Choose a supported character action.")
		}
		return
	}
	id, action, _ := strings.Cut(strings.TrimPrefix(path, "/"), "/")
	if !characterID.MatchString(id) {
		accessError(w, ErrNotFound)
		return
	}
	switch {
	case r.Method == http.MethodGet && action == "":
		c, err := scanCharacter(s.Pool.QueryRow(r.Context(), "SELECT "+characterColumns+` FROM syntelle_accounts.character_creations WHERE id=$1 AND account_id=$2 AND deleted_at IS NULL`, id, p.AccountID))
		if err != nil {
			accessError(w, err)
			return
		}
		httpapi.WriteJSON(w, 200, c)
	case r.Method == http.MethodGet && action == "portrait":
		var data []byte
		var mime string
		err := s.Pool.QueryRow(r.Context(), `SELECT source_bytes,mime_type FROM syntelle_accounts.character_creations WHERE id=$1 AND account_id=$2 AND deleted_at IS NULL AND expires_at>$3 AND source_bytes IS NOT NULL`, id, p.AccountID, s.cfg.Now()).Scan(&data, &mime)
		if errors.Is(err, pgx.ErrNoRows) {
			accessError(w, ErrNotFound)
			return
		}
		if err != nil {
			accessError(w, err)
			return
		}
		w.Header().Set("Content-Type", mime)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		_, _ = w.Write(data)
	case r.Method == http.MethodDelete && action == "":
		tag, err := s.Pool.Exec(r.Context(), `UPDATE syntelle_accounts.character_creations SET source_bytes=NULL,deleted_at=$3,updated_at=$3,
status=CASE WHEN status IN ('queued','processing') THEN 'failed' ELSE status END,
failure_code=CASE WHEN status IN ('queued','processing') THEN 'cancelled' ELSE failure_code END
WHERE id=$1 AND account_id=$2 AND deleted_at IS NULL`, id, p.AccountID, s.cfg.Now())
		if err != nil {
			accessError(w, err)
			return
		}
		if tag.RowsAffected() != 1 {
			accessError(w, ErrNotFound)
			return
		}
		if s.OnCharacterDeleted != nil {
			s.OnCharacterDeleted(p.AccountID, id)
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		accessError(w, ErrNotFound)
	}
}

type creationError struct {
	status        int
	code, message string
}

func (e *creationError) Error() string { return e.code }

func creationFailure(w http.ResponseWriter, err error) {
	var e *creationError
	if errors.As(err, &e) {
		httpapi.WriteError(w, e.status, e.code, e.message)
		return
	}
	accessError(w, err)
}

func readPortrait(w http.ResponseWriter, r *http.Request) (string, []byte, error) {
	invalid := &creationError{400, "invalid_request", "Send one character name and one portrait."}
	tooLarge := &creationError{413, "file_too_large", "Choose a portrait no larger than 5 MB."}
	r.Body = http.MaxBytesReader(w, r.Body, maxPortraitBytes+32768)
	reader, err := r.MultipartReader()
	if err != nil {
		return "", nil, invalid
	}
	var name string
	var photo []byte
	seen := map[string]bool{}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		var limit *http.MaxBytesError
		if errors.As(err, &limit) {
			return "", nil, tooLarge
		}
		if err != nil {
			return "", nil, invalid
		}
		key := part.FormName()
		if seen[key] {
			return "", nil, invalid
		}
		seen[key] = true
		switch key {
		case "name":
			data, err := io.ReadAll(io.LimitReader(part, 401))
			name = strings.TrimSpace(string(data))
			if err != nil || part.FileName() != "" || len(data) > 400 || !utf8.Valid(data) || name == "" || utf8.RuneCountInString(name) > 100 || strings.IndexFunc(name, unicode.IsControl) >= 0 {
				return "", nil, &creationError{400, "invalid_name", "Use a name between 1 and 100 characters."}
			}
		case "image":
			photo, err = io.ReadAll(io.LimitReader(part, maxPortraitBytes+1))
			if errors.As(err, &limit) || len(photo) > maxPortraitBytes {
				return "", nil, tooLarge
			}
			if err != nil || len(photo) == 0 || part.FileName() == "" {
				return "", nil, invalid
			}
		default:
			return "", nil, invalid
		}
	}
	if name == "" || len(photo) == 0 {
		return "", nil, invalid
	}
	return name, photo, nil
}

type portraitMetadata struct {
	Format string `json:"format"`
	Mime   string `json:"mime_type"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Size   int    `json:"size_bytes"`
}

func (s *Service) validatePortrait(ctx context.Context, photo []byte) (portraitMetadata, error) {
	var m portraitMetadata
	unavailable := &creationError{503, "processor_unavailable", "Portrait validation is temporarily unavailable."}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("image", "portrait")
	if err != nil {
		return m, unavailable
	}
	if _, err = part.Write(photo); err != nil {
		return m, unavailable
	}
	if err = writer.Close(); err != nil {
		return m, unavailable
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(s.cfg.ProcessorURL, "/")+"/internal/v1/images/validate", &body)
	if err != nil {
		return m, unavailable
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if httpapi.Cloud() {
		req.Header.Set("Authorization", "Bearer "+os.Getenv("SYNTELLE_PROCESSOR_TOKEN"))
	}
	res, err := s.cfg.HTTPClient.Do(req)
	if err != nil {
		return m, unavailable
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 8193))
	if err != nil || len(data) > 8192 {
		return m, unavailable
	}
	switch res.StatusCode {
	case 415:
		return m, &creationError{415, "unsupported_format", "Choose a JPEG, PNG, or WebP portrait."}
	case 422:
		return m, &creationError{422, "invalid_image", "Choose an intact portrait with supported dimensions."}
	case 200:
		expected := map[string]string{"JPEG": "image/jpeg", "PNG": "image/png", "WEBP": "image/webp"}
		if json.Unmarshal(data, &m) == nil && m.Mime != "" && expected[m.Format] == m.Mime && m.Mime == http.DetectContentType(photo) && m.Size == len(photo) && m.Width > 0 && m.Height > 0 {
			return m, nil
		}
	}
	return m, unavailable
}

func (s *Service) createCharacter(w http.ResponseWriter, r *http.Request, account string) {
	select {
	case s.characterUploads <- struct{}{}:
		defer func() { <-s.characterUploads }()
	default:
		httpapi.WriteError(w, 429, "preparation_busy", "Portrait preparation is busy. Try again shortly.")
		return
	}
	name, photo, err := readPortrait(w, r)
	if err != nil {
		creationFailure(w, err)
		return
	}
	hash := sha256.Sum256(photo)
	source := hex.EncodeToString(hash[:])
	key := r.Header.Get("Idempotency-Key")
	if key == "" {
		key = digest(name + "\x00" + source)
	}
	if !creationRequestKey.MatchString(key) {
		httpapi.WriteError(w, 400, "invalid_request_key", "Use a valid request identifier.")
		return
	}
	if previous, err := lookupCharacterRequest(r.Context(), s.Pool, account, name, key, source); err != nil {
		creationFailure(w, err)
		return
	} else if previous != nil {
		httpapi.WriteJSON(w, http.StatusOK, previous)
		return
	}
	m, err := s.validatePortrait(r.Context(), photo)
	if err != nil {
		creationFailure(w, err)
		return
	}
	c, fresh, err := s.reserveCharacter(r.Context(), account, name, key, source, photo, m)
	if err != nil {
		creationFailure(w, err)
		return
	}
	status := 200
	if fresh {
		status = http.StatusAccepted
	}
	httpapi.WriteJSON(w, status, c)
}

type characterQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func lookupCharacterRequest(ctx context.Context, q characterQuerier, account, name, key, source string) (*characterRecord, error) {
	var deleted bool
	var c characterRecord
	err := q.QueryRow(ctx, "SELECT "+characterColumns+",deleted_at IS NOT NULL FROM syntelle_accounts.character_creations WHERE account_id=$1 AND request_key=$2", account, key).
		Scan(&c.ID, &c.Name, &c.SourceSHA256, &c.MimeType, &c.Width, &c.Height, &c.Status, &c.PackageHash, &c.FailureCode, &c.CreatedAt, &c.UpdatedAt, &c.ExpiresAt, &c.PortraitAvailable, &deleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if deleted || c.Name != name || c.SourceSHA256 != source {
		return nil, &creationError{409, "request_conflict", "This request identifier was already used."}
	}
	return &c, nil
}

func (s *Service) reserveCharacter(ctx context.Context, account, name, key, source string, photo []byte, m portraitMetadata) (characterRecord, bool, error) {
	var c characterRecord
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return c, false, err
	}
	defer tx.Rollback(ctx)
	// Serialize the short admission transaction, not image decoding or preparation.
	// The ready limit includes tombstones so deletion cannot reset daily capacity.
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(187200047)"); err != nil {
		return c, false, err
	}
	if previous, err := lookupCharacterRequest(ctx, tx, account, name, key, source); err != nil {
		return c, false, err
	} else if previous != nil {
		return *previous, false, nil
	}
	now := s.cfg.Now()
	var recent, active, busy bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT FROM syntelle_accounts.character_creations WHERE account_id=$1 AND ready_at>$2),
EXISTS(SELECT FROM syntelle_accounts.character_creations WHERE account_id=$1 AND status IN ('queued','processing') AND deleted_at IS NULL),
(SELECT count(*)>=16 FROM syntelle_accounts.character_creations WHERE status IN ('queued','processing') AND deleted_at IS NULL)
OR (SELECT coalesce(sum(octet_length(source_bytes)),0)+$3::bigint>256000000 FROM syntelle_accounts.character_creations WHERE source_bytes IS NOT NULL)`, account, now.Add(-24*time.Hour), len(photo)).Scan(&recent, &active, &busy)
	if err != nil {
		return c, false, err
	}
	if recent {
		return c, false, &creationError{429, "creation_limit", "You can finish one new character every 24 hours."}
	}
	if active {
		return c, false, &creationError{409, "preparation_active", "You already have a character being prepared."}
	}
	if busy {
		return c, false, &creationError{429, "preparation_busy", "Portrait preparation is busy. Try again later."}
	}
	id := "char_" + digest(randomID())[:32]
	c, err = scanCharacter(tx.QueryRow(ctx, `INSERT INTO syntelle_accounts.character_creations
(id,account_id,request_key,name,source_sha256,source_bytes,mime_type,width,height,status,created_at,updated_at,expires_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'queued',$10,$10,$11) RETURNING `+characterColumns, id, account, key, name, source, photo, m.Mime, m.Width, m.Height, now, now.Add(24*time.Hour)))
	if err != nil {
		return c, false, err
	}
	return c, true, tx.Commit(ctx)
}
