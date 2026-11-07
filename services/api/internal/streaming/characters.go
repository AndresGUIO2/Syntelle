package streaming

import (
	"context"
	"encoding/hex"
	"errors"

	"syntelle/api/internal/accounts"
	"syntelle/render-contract"
)

func (s *Service) resolvePackage(ctx context.Context, p accounts.Principal, character string) (string, error) {
	if character == accounts.Defaults().Character {
		return s.config.PackageHash, nil
	}
	if !rendercontract.ValidCharacterID(character) || p.Kind != "account" || p.AccountID == "" || s.config.ResolveCharacter == nil {
		return "", errors.New("prepared character unavailable")
	}
	hash, err := s.config.ResolveCharacter(ctx, p.AccountID, character)
	if err != nil {
		return "", err
	}
	bytes, err := hex.DecodeString(hash)
	if err != nil || len(bytes) != 32 || hex.EncodeToString(bytes) != hash {
		return "", errors.New("invalid prepared character identity")
	}
	return hash, nil
}

func normalizeBasicSettings(settings *accounts.Settings) {
	settings.EmotionBaseline = rendercontract.EmotionBaseline{State: "neutral"}
	settings.EmotionRegulation = rendercontract.EmotionRegulation{}
}

func (s *recordStore) characterName(ctx context.Context, account, character, hash string) (string, error) {
	var name string
	err := s.pool.QueryRow(ctx, `SELECT name FROM syntelle_accounts.character_creations WHERE id=$1 AND account_id=$2 AND package_hash=$3 AND status='ready' AND deleted_at IS NULL`, character, account, hash).Scan(&name)
	return name, err
}

// Called only after the ownership-scoped tombstone commits. Holding characterMu
// closes the admission/delete race without serializing ordinary stream commands.
func (s *Service) CharacterDeleted(account, character string) {
	s.characterMu.Lock()
	defer s.characterMu.Unlock()
	s.mu.Lock()
	ids := []string{}
	for id, session := range s.sessions {
		if session.principal.AccountID == account && session.settings.Character == character {
			ids = append(ids, id)
		}
	}
	s.mu.Unlock()
	for _, id := range ids {
		s.closeSession(id, "character deleted")
	}
}

// Existing in-memory fixtures predate per-session bindings. Production creates
// always set packageHash after account authorization.
func (s *Service) sessionPackageHash(session *streamSession) string {
	if session.packageHash != "" {
		return session.packageHash
	}
	return s.config.PackageHash
}

func (s *recordStore) matchesPausedCharacter(ctx context.Context, id, principal, character, hash string) error {
	var matches bool
	err := s.pool.QueryRow(ctx, `SELECT settings->>'character'=$3 AND package_hash=$4 FROM syntelle_streaming.sessions WHERE id=$1 AND principal_id=$2 AND state='paused' AND closed_at IS NOT NULL`, id, principal, character, hash).Scan(&matches)
	if err != nil {
		return err
	}
	if !matches {
		return errors.New("paused character mismatch")
	}
	return nil
}
