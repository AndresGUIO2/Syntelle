package streaming

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func (s *Service) ensureWorker(ctx context.Context) error {
	s.bootMu.Lock()
	defer s.bootMu.Unlock()
	s.mu.Lock()
	ready := s.worker != nil
	s.mu.Unlock()
	if ready {
		return nil
	}
	endpoint := os.Getenv("SYNTELLE_MODAL_LAUNCH_URL")
	u, e := url.Parse(endpoint)
	if e != nil || u.Scheme != "https" || !strings.HasSuffix(u.Hostname(), ".modal.run") || u.User != nil {
		return errors.New("invalid Modal launcher URL")
	}
	id, e := randomID("allocation_")
	if e != nil {
		return e
	}
	// A persisted lease prevents ambiguous HTTP results/API restarts from
	// automatically purchasing a second invocation. No provider budget is reset.
	result, e := s.store.pool.Exec(ctx, `INSERT INTO syntelle_streaming.worker_allocation(singleton,id,expires_at) VALUES(true,$1,now()+interval '110 seconds') ON CONFLICT(singleton) DO UPDATE SET id=excluded.id,expires_at=excluded.expires_at WHERE worker_allocation.expires_at<now()`, id)
	if e != nil {
		return e
	}
	if result.RowsAffected() == 1 {
		body, _ := json.Marshal(map[string]string{"allocation_id": id})
		req, e := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
		if e != nil {
			return e
		}
		req.Header.Set("Authorization", "Bearer "+os.Getenv("SYNTELLE_MODAL_LAUNCH_TOKEN"))
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		if s.config.LaunchClient != nil {
			client = s.config.LaunchClient
		}
		res, e := client.Do(req)
		if e != nil {
			return errors.New("renderer launch outcome uncertain; not retried")
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
		res.Body.Close()
		if res.StatusCode != 202 {
			return errors.New("renderer launch rejected; not retried")
		}
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			s.mu.Lock()
			ready = s.worker != nil
			s.mu.Unlock()
			if ready {
				return nil
			}
		}
	}
}
func (s *Service) deliverCreate(session *streamSession) bool {
	return s.toWorker(envelope("create", session.id, map[string]any{"settings": session.settings, "package_hash": s.config.PackageHash, "width": 720, "height": 960, "fps": 30}))
}
func (s *Service) bootSession(parent context.Context, session *streamSession) {
	defer s.wg.Done()
	ctx, cancel := context.WithTimeout(parent, 90*time.Second)
	defer cancel()
	if s.ensureWorker(ctx) != nil {
		s.toPlayer(session, envelope("error", session.id, map[string]string{"message": "The renderer could not start. Start a new session."}))
		s.closeSession(session.id, "renderer startup failed")
		return
	}
	s.mu.Lock()
	active := s.sessions[session.id] == session
	s.mu.Unlock()
	if active && !s.deliverCreate(session) {
		s.closeSession(session.id, "renderer disconnected")
	}
}
