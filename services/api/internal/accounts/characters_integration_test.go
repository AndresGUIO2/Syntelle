//go:build integration

package accounts

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"syntelle/api/internal/httpapi"
)

const charactersPath = "/api/v1/characters"

func fixturePortrait(t *testing.T, f *accountFixture) []byte {
	t.Helper()
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 32, 48))); err != nil {
		t.Fatal(err)
	}
	processor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/v1/images/validate" {
			http.NotFound(w, r)
			return
		}
		reader, err := r.MultipartReader()
		if err != nil {
			http.Error(w, "missing multipart", 400)
			return
		}
		part, err := reader.NextPart()
		if err != nil || part.FormName() != "image" {
			http.Error(w, "missing image", 400)
			return
		}
		data, err := io.ReadAll(part)
		if err != nil {
			http.Error(w, "invalid source", 400)
			return
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			http.Error(w, "invalid PNG", 422)
			return
		}
		httpapi.WriteJSON(w, 200, map[string]any{"format": "PNG", "mime_type": "image/png", "width": img.Bounds().Dx(), "height": img.Bounds().Dy(), "size_bytes": len(data)})
	}))
	t.Cleanup(processor.Close)
	f.s.cfg.ProcessorURL = processor.URL
	return encoded.Bytes()
}

func portraitUpload(f *accountFixture, cookie *http.Cookie, name, key string, data []byte, origin string) *httptest.ResponseRecorder {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("name", name)
	part, _ := writer.CreateFormFile("image", "untrusted-label.jpeg")
	_, _ = part.Write(data)
	_ = writer.Close()
	r := httptest.NewRequest("POST", "http://127.0.0.1:5173"+charactersPath, &body)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	r.Header.Set("Origin", origin)
	r.Header.Set("X-Syntelle-Request", "1")
	r.Header.Set("Idempotency-Key", key)
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	return w
}

func TestPersonalCharacterCreationOwnershipAndLifecycle(t *testing.T) {
	f := newAccountFixture(t)
	data := fixturePortrait(t, f)
	owner, a := f.signin(f.guest(), "portrait-"+randomID()+"@example.com")
	other, b := f.signin(f.guest(), "separate-"+randomID()+"@example.com")
	const origin = "http://127.0.0.1:5173"
	fixtureJSON(t, portraitUpload(f, f.guest(), "My portrait", "first", data, origin), 401)
	fixtureJSON(t, portraitUpload(f, owner, "My portrait", "first", data, "https://untrusted.example"), 403)
	fixtureJSON(t, portraitUpload(f, owner, "Bad portrait", "bad", []byte("<svg></svg>"), origin), 422)
	q := fixtureJSON(t, portraitUpload(f, owner, "My portrait", "first", data, origin), 202)
	id := q["id"].(string)
	path := charactersPath + "/" + id
	sum := sha256.Sum256(data)
	if !characterID.MatchString(id) || q["status"] != "queued" || q["source_sha256"] != hex.EncodeToString(sum[:]) || q["mime_type"] != "image/png" || q["portrait_available"] != true {
		t.Fatal("queued source mismatch", q)
	}
	// Previous submissions remain recoverable even when the validator goes offline.
	validator := f.s.cfg.ProcessorURL
	f.s.cfg.ProcessorURL = "http://127.0.0.1:1"
	if key := fixtureJSON(t, portraitUpload(f, owner, "My portrait", "first", data, origin), 200)["id"]; key != id {
		t.Fatal("duplicate request created another character")
	}
	fixtureJSON(t, portraitUpload(f, owner, "Altered name", "first", data, origin), 409)
	f.s.cfg.ProcessorURL = validator
	fixtureJSON(t, portraitUpload(f, owner, "Second", "second", data, origin), 409)
	if items := fixtureJSON(t, f.request("GET", charactersPath, nil, owner, nil), 200)["items"].([]any); len(items) != 1 {
		t.Fatal("owner collection missing existing creation")
	}
	if items := fixtureJSON(t, f.request("GET", charactersPath, nil, other, nil), 200)["items"].([]any); len(items) != 0 {
		t.Fatal("collection crossed account boundary")
	}
	for _, path := range []string{path, path + "/portrait"} {
		fixtureJSON(t, f.request("GET", path, nil, other, nil), 404)
	}
	fixtureJSON(t, f.request("DELETE", path, nil, other, nil), 404)
	preview := f.request("GET", path+"/portrait", nil, owner, nil)
	if preview.Code != 200 || preview.Header().Get("Content-Type") != "image/png" || !bytes.Equal(preview.Body.Bytes(), data) {
		t.Fatal("private preview is not the exact original")
	}
	ctx := t.Context()
	if _, err := f.s.ResolveCharacter(ctx, a["id"].(string), id); !errors.Is(err, ErrNotFound) {
		t.Fatal("unprepared portrait became playable", err)
	}
	// Simulate only the trusted processor completion, keeping package identity
	// entirely outside user inputs. Actual preparation is covered separately.
	hash := strings.Repeat("b", 64)
	if _, err := f.pool.Exec(ctx, "UPDATE syntelle_accounts.character_creations SET status='ready',package_hash=$2,ready_at=$3 WHERE id=$1", id, hash, f.now); err != nil {
		t.Fatal(err)
	}
	if got, err := f.s.ResolveCharacter(ctx, a["id"].(string), id); err != nil || got != hash {
		t.Fatal("ready package did not resolve")
	}
	if _, err := f.s.ResolveCharacter(ctx, b["id"].(string), id); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign ready package resolved")
	}
	settings := Defaults()
	settings.Character = id
	input := map[string]any{"name": "Own custom settings", "settings": settings}
	saved := fixtureJSON(t, f.request("POST", "/api/v1/configurations", input, owner, nil), 201)
	if saved["package_hash"] != hash {
		t.Fatal("personal custom settings bound to another package", saved["package_hash"])
	}
	fixtureJSON(t, f.request("POST", "/api/v1/configurations", input, other, nil), 404)
	workspace := fixtureJSON(t, f.request("POST", "/api/v1/workspaces", map[string]string{"name": "Team"}, owner, nil), 201)
	shared := "/api/v1/workspaces/" + workspace["id"].(string) + "/configurations"
	fixtureJSON(t, f.request("POST", shared, input, owner, nil), 400)
	fixtureJSON(t, f.request("POST", shared+"/copy", map[string]any{"configuration_id": saved["id"]}, owner, nil), 404)
	deletedCallbacks := 0
	f.s.OnCharacterDeleted = func(account, character string) {
		deletedCallbacks++
		if account != a["id"].(string) || character != id {
			t.Fatal("deletion callback crossed ownership")
		}
		if _, err := f.s.ResolveCharacter(ctx, account, character); !errors.Is(err, ErrNotFound) {
			t.Fatal("deletion callback preceded durable tombstone")
		}
	}
	if w := f.request("DELETE", path, nil, owner, nil); w.Code != 204 {
		t.Fatal("delete", w.Code)
	}
	if deletedCallbacks != 1 {
		t.Fatal("missing deletion callback")
	}
	fixtureJSON(t, f.request("DELETE", path, nil, other, nil), 404)
	if deletedCallbacks != 1 {
		t.Fatal("foreign deletion invoked callback")
	}
	fixtureJSON(t, f.request("GET", path, nil, owner, nil), 404)
	fixtureJSON(t, f.request("GET", path+"/portrait", nil, owner, nil), 404)
	if _, err := f.s.ResolveCharacter(ctx, a["id"].(string), id); !errors.Is(err, ErrNotFound) {
		t.Fatal("deleted package resolved")
	}
	fixtureJSON(t, portraitUpload(f, owner, "Another", "new-key", data, origin), 429)
}

func TestCharacterPreparationReservationRacesFailureAndExpiry(t *testing.T) {
	f := newAccountFixture(t)
	data := fixturePortrait(t, f)
	owner, _ := f.signin(f.guest(), "portrait-race-"+randomID()+"@example.com")
	const origin = "http://127.0.0.1:5173"
	responses := make(chan *httptest.ResponseRecorder, 2)
	var wait sync.WaitGroup
	for _, key := range []string{"one", "two"} {
		wait.Add(1)
		go func() { defer wait.Done(); responses <- portraitUpload(f, owner, "Concurrent", key, data, origin) }()
	}
	wait.Wait()
	close(responses)
	var id string
	conflicts := 0
	for result := range responses {
		switch result.Code {
		case 202:
			id = fixtureJSON(t, result, 202)["id"].(string)
		case 409:
			conflicts++
		default:
			t.Fatal(result.Code, result.Body.String())
		}
	}
	if id == "" || conflicts != 1 {
		t.Fatal("simultaneous requests did not reserve exactly one slot")
	}
	if _, err := f.pool.Exec(t.Context(), "UPDATE syntelle_accounts.character_creations SET status='failed',failure_code='unsuitable_portrait' WHERE id=$1", id); err != nil {
		t.Fatal(err)
	}
	active := fixtureJSON(t, portraitUpload(f, owner, "Replacement", "replacement", data, origin), 202)["id"].(string)
	if _, err := f.pool.Exec(t.Context(), "UPDATE syntelle_accounts.character_creations SET expires_at=$2 WHERE id=$1", active, f.now.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	record := fixtureJSON(t, f.request("GET", charactersPath+"/"+active, nil, owner, nil), 200)
	if record["status"] != "failed" || record["failure_code"] != "source_expired" || record["portrait_available"] != false {
		t.Fatal("expired source still in preparation", record)
	}
	fixtureJSON(t, f.request("GET", charactersPath+"/"+active+"/portrait", nil, owner, nil), 404)
	fixtureJSON(t, portraitUpload(f, owner, "Finally", "after-expiry", data, origin), 202)
}
