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
	"syntelle/api/internal/httpapi"
	"time"
)

const cloudWorkerCapacity = 2

// Caller holds s.mu. A container with two assigned sessions is full even if
// its GPU has free memory; real T4 throughput must qualify a higher target.
func (s *Service) assignCloudWorkerLocked(session *streamSession, now time.Time) bool {
	chosen := ""
	for id, worker := range s.workers {
		if now.Sub(worker.started) > 24*time.Minute {
			continue
		}
		assigned := 0
		for _, current := range s.sessions {
			if current.workerID == id {
				assigned++
			}
		}
		if assigned < cloudWorkerCapacity && (chosen == "" || id < chosen) {
			chosen = id
		}
	}
	if chosen == "" {
		return false
	}
	session.workerID = chosen
	return true
}

func modalLaunchEndpoint() (string, error) {
	endpoint := os.Getenv("SYNTELLE_MODAL_LAUNCH_URL")
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || !strings.HasSuffix(u.Hostname(), ".modal.run") || u.User != nil {
		return "", errors.New("invalid Modal launcher URL")
	}
	return endpoint, nil
}

func (s *Service) launchModal(ctx context.Context, endpoint, id string) error {
	body, _ := json.Marshal(map[string]string{"allocation_id": id})
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("SYNTELLE_MODAL_LAUNCH_TOKEN"))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	if s.config.LaunchClient != nil {
		client = s.config.LaunchClient
	}
	res, err := client.Do(req)
	if err != nil {
		return errors.New("renderer launch outcome uncertain; not retried")
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
	res.Body.Close()
	if res.StatusCode != 202 {
		return errors.New("renderer launch rejected; not retried")
	}
	return nil
}

func (s *Service) ensureWorker(ctx context.Context) error {
	s.bootMu.Lock()
	defer s.bootMu.Unlock()
	s.mu.Lock()
	ready := s.worker != nil
	s.mu.Unlock()
	if ready {
		return nil
	}
	endpoint, e := modalLaunchEndpoint()
	if e != nil {
		return e
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
		if e = s.launchModal(ctx, endpoint, id); e != nil {
			return e
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

// A session is bound to one identified worker before its create command.
// A pending durable lease is awaited rather than launched again after an
// ambiguous HTTP result or an API restart.
func (s *Service) ensureCloudWorker(ctx context.Context, session *streamSession) error {
	endpoint, err := modalLaunchEndpoint()
	if err != nil {
		return err
	}
	s.bootMu.Lock()
	defer s.bootMu.Unlock()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		s.mu.Lock()
		if s.sessions[session.id] != session || s.closing {
			s.mu.Unlock()
			return errors.New("stream session closed before renderer assignment")
		}
		connected := make(map[string]bool, len(s.workers))
		for id := range s.workers {
			connected[id] = true
		}
		if s.assignCloudWorkerLocked(session, time.Now()) {
			s.mu.Unlock()
			return nil
		}
		s.mu.Unlock()
		rows, err := s.store.pool.Query(ctx, "SELECT id FROM syntelle_streaming.worker_allocations WHERE expires_at>now()")
		if err != nil {
			return err
		}
		pending := false
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				break
			}
			if !connected[id] {
				pending = true
			}
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return err
		}
		if !pending {
			id, err := randomID("allocation_")
			if err != nil {
				return err
			}
			if _, err = s.store.pool.Exec(ctx, "INSERT INTO syntelle_streaming.worker_allocations(id,expires_at) VALUES($1,now()+interval '110 seconds')", id); err != nil {
				return err
			}
			if err = s.launchModal(ctx, endpoint, id); err != nil {
				return err
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
func (s *Service) deliverCreate(session *streamSession) bool {
	width, height := 1440, 960
	if httpapi.Cloud() {
		width, height = 1080, 720
	}
	return s.toWorker(envelope("create", session.id, map[string]any{"settings": session.settings, "emotion_policy_revision": session.emotionPolicyRevision, "package_hash": s.config.PackageHash, "width": width, "height": height, "fps": 30}))
}
func (s *Service) bootSession(parent context.Context, session *streamSession) {
	defer s.wg.Done()
	ctx, cancel := context.WithTimeout(parent, 90*time.Second)
	defer cancel()
	var err error
	if httpapi.Cloud() {
		err = s.ensureCloudWorker(ctx, session)
	} else {
		err = s.ensureWorker(ctx)
	}
	if err != nil {
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
