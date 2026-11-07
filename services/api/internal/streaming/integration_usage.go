package streaming

import (
	"context"
	"encoding/json"
	"time"
)

func (s *recordStore) allocateIntegration(ctx context.Context, session *streamSession, hash string) error {
	raw, err := json.Marshal(session.settings)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO syntelle_streaming.sessions(id,conversation_id,principal_id,principal_kind,key_id,settings,package_hash,state,workspace_id,integration_id,publication_id,integration_revision) VALUES($1,$2,$3,$4,$5,$6,$7,'starting',$8,$9,$10,$11)`, session.id, session.cid, session.principal.ID, session.principal.Kind, session.principal.KeyID, raw, hash, session.principal.WorkspaceID, session.principal.IntegrationID, session.principal.PublicationID, session.principal.IntegrationRevision)
	return err
}
func (s *Service) startIntegrationUsage(session *streamSession) bool {
	if session.principal.IntegrationID == "" {
		return true
	}
	s.mu.Lock()
	current := s.sessions[session.id] == session
	started := session.usageStarted
	s.mu.Unlock()
	if !current {
		return false
	}
	if started {
		return true
	}
	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Second)
	err := s.config.StartIntegrationUsage(ctx, session.id)
	cancel()
	if err != nil {
		s.closeSession(session.id, "workspace usage unavailable")
		return false
	}
	s.mu.Lock()
	session.usageStarted = true
	s.mu.Unlock()
	return true
}
func (s *Service) renewIntegrationUsage(now time.Time) {
	s.mu.Lock()
	pending := []*streamSession{}
	for _, session := range s.sessions {
		if session.principal.IntegrationID != "" && !now.Before(session.nextUsageRenewal) {
			session.nextUsageRenewal = now.Add(10 * time.Second)
			pending = append(pending, session)
		}
	}
	s.mu.Unlock()
	for _, session := range pending {
		ctx, cancel := context.WithTimeout(s.ctx, 2*time.Second)
		err := s.config.ValidatePrincipal(ctx, session.principal)
		if err == nil {
			_, err = s.config.RenewIntegrationUsage(ctx, session.id)
		}
		cancel()
		if err != nil {
			s.closeSession(session.id, "workspace usage expired")
		}
	}
}
