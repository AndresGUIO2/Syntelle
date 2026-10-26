//go:build integration

package streaming

// This host is compiled only into the integration test binary. It has no route,
// flag or provider implementation in the production executable.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"syntelle/api/internal/accounts"
	"syntelle/api/internal/conversation"
)

const fixtureWorkerToken = "syntelle-offline-worker-fixture-49"
const fixtureControlToken = "syntelle-offline-control-fixture"

type capturedFixtureMail struct {
	mu                     sync.Mutex
	email, code, challenge string
	sent                   int
}

func (m *capturedFixtureMail) SendCode(_ context.Context, email, code, challenge string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.email, m.code, m.challenge = email, code, challenge
	m.sent++
	return "offline-fixture-mail-" + challenge, nil
}

type cachedVoiceFixture struct {
	store   *conversation.Store
	payload map[string]any
	audio   []byte
	hash    string
	calls   atomic.Int32
}

func (v *cachedVoiceFixture) Prepare(ctx context.Context, text, language, voice, id string) (any, error) {
	v.calls.Add(1)
	cid, ok := conversation.IDs(ctx)
	if !ok {
		return nil, errors.New("fixture speech requires conversation binding")
	}
	key := id
	if phraseKey, ok := conversation.PhraseSpeechKey(ctx); ok {
		key = phraseKey
	}
	op, err := v.store.Reserve(ctx, "fixture-native-speech", key+"-cached", v.hash, cid, id, map[string]any{"text": text, "language": language, "voice": voice, "fixture": true, "source_audio_sha256": v.hash}, false)
	if err != nil {
		return nil, err
	}
	if err = v.store.Finish(ctx, op.ID, "received", v.payload, map[string]int{"provider_calls": 0}, map[string]any{"fixture_cached_speech": true, "provider_calls": 0}, v.audio); err != nil {
		return nil, err
	}
	return v.payload, nil
}
func (v *cachedVoiceFixture) PrepareParts(ctx context.Context, text, language, voice, id string, emit func(conversation.SpeechPart) error) (any, error) {
	payload, err := v.Prepare(ctx, text, language, voice, id)
	if err != nil {
		return nil, err
	}
	if err = emit(conversation.SpeechPart{Speech: payload, Text: text, Index: 0, Final: true}); err != nil {
		return nil, err
	}
	return payload, nil
}

type hostRecognitionFixture struct{ calls atomic.Int32 }

func (r *hostRecognitionFixture) Run(ctx context.Context, _ string, pcm <-chan []byte, emit func(conversation.RecognitionEvent)) error {
	r.calls.Add(1)
	frames := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case frame, ok := <-pcm:
			if !ok {
				return nil
			}
			if len(frame) == 0 {
				continue
			}
			frames++
			if frames == 1 {
				emit(conversation.RecognitionEvent{Type: "start"})
			}
			if frames == 10 {
				emit(conversation.RecognitionEvent{Type: "partial", Text: "Hello from the offline microphone fixture."})
			}
			if frames == 15 {
				emit(conversation.RecognitionEvent{Type: "final", ID: "fixture-native-" + strings.ReplaceAll(time.Now().Format("150405.000000000"), ".", ""), Text: "Hello from the offline microphone fixture."})
			}
		}
	}
}

func TestStreamingFixtureHost(t *testing.T) {
	if os.Getenv("SYNTELLE_ENABLE_STREAM_FIXTURE") != "1" {
		t.Skip("explicit long-running fixture host disabled")
	}
	store := streamingTestStore(t)
	path := os.Getenv("SYNTELLE_STREAMING_FIXTURE_SPEECH")
	if path == "" {
		path = "../../../../.tools/accounts-streaming-49/fixtures/cached-speech.json"
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal("read approved cached speech fixture:", err)
	}
	var payload map[string]any
	var audio struct {
		Audio []byte `json:"audio_base64"`
	}
	if json.Unmarshal(data, &payload) != nil || json.Unmarshal(data, &audio) != nil || len(audio.Audio) < 44 {
		t.Fatal("cached fixture has no native WAV")
	}
	audioHash := sha256.Sum256(audio.Audio)
	voice := &cachedVoiceFixture{store: store, payload: payload, audio: audio.Audio, hash: hex.EncodeToString(audioHash[:])}
	manifestPath := os.Getenv("SYNTELLE_STREAMING_FIXTURE_MANIFEST")
	if manifestPath == "" {
		manifestPath = "../../../../.tools/accounts-streaming-49/package-manifest.json"
	}
	manifestBytes, err := os.ReadFile(filepath.Clean(manifestPath))
	if err != nil {
		t.Fatal("read package provenance:", err)
	}
	var manifest struct {
		Hash string `json:"package_hash"`
	}
	if json.Unmarshal(manifestBytes, &manifest) != nil || len(manifest.Hash) != 64 {
		t.Fatal("invalid package provenance")
	}
	model := &streamModelFixture{}
	recognition := &hostRecognitionFixture{}
	mailer := &capturedFixtureMail{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	accountsService, err := accounts.Open(ctx, store.Pool, accounts.Config{Secret: "syntelle-fixture-auth-secret-only-20261021", Origin: "http://127.0.0.1:5173", Mailer: mailer, PackageHash: manifest.Hash})
	if err != nil {
		t.Fatal(err)
	}
	conversations := conversation.New(store, model, voice, recognition)
	if os.Getenv("SYNTELLE_STREAMING_FIXTURE_EXPRESSIONS") == "1" {
		conversations.Model = &expressionDirectorFixture{streamModelFixture: model}
		path := os.Getenv("SYNTELLE_STREAMING_FIXTURE_PERFORMANCE")
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal("read offline expression profile:", readErr)
		}
		var profile conversation.PerformanceProfile
		if json.Unmarshal(data, &profile) != nil || profile.PackageHash != manifest.Hash {
			t.Fatal("offline expression profile binding differs")
		}
		conversations.Performance = &profile
	}
	if os.Getenv("SYNTELLE_STREAMING_FIXTURE_EMOTIONS") == "1" {
		conversations.Model = &emotionDirectorFixture{streamModelFixture: model}
	}
	if os.Getenv("SYNTELLE_STREAMING_FIXTURE_GESTURES") == "1" {
		conversations.Model = &gestureDirectorFixture{streamModelFixture: model}
	}
	service, err := Open(ctx, store.Pool, conversations, accountsService.Resolve, Config{WorkerToken: fixtureWorkerToken, MaximumSessions: 2, PackageHash: manifest.Hash, ValidatePrincipal: accountsService.ValidatePrincipal})
	if err != nil {
		t.Fatal(err)
	}
	accountsService.OnRevoked = service.Revoke
	stop := make(chan struct{})
	var stopOnce sync.Once
	control := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/__fixture__/") {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+fixtureControlToken {
			http.Error(w, "fixture control authorization required", 403)
			return
		}
		switch r.URL.Path {
		case "/__fixture__/directions":
			if model, ok := conversations.Model.(*emotionDirectorFixture); ok {
				model.mu.Lock()
				defer model.mu.Unlock()
				writeJSON(w, 200, model.directions)
			} else {
				writeJSON(w, 200, []string{})
			}
		case "/__fixture__/emotion-evidence":
			rows, err := store.Pool.Query(r.Context(), "SELECT jsonb_build_object('id',id,'kind',kind,'status',status,'request',request,'result',result,'diagnostics',diagnostics) FROM syntelle_conversation.operations WHERE kind IN ('emotion-activation','emotion-decision','speech-delivery') ORDER BY id")
			if err != nil {
				http.Error(w, "fixture evidence query failed", 500)
				return
			}
			defer rows.Close()
			values := []json.RawMessage{}
			for rows.Next() {
				var value json.RawMessage
				if rows.Scan(&value) != nil {
					http.Error(w, "fixture evidence scan failed", 500)
					return
				}
				values = append(values, value)
			}
			if rows.Err() != nil {
				http.Error(w, "fixture evidence rows failed", 500)
				return
			}
			writeJSON(w, 200, values)
		case "/__fixture__/mail":
			mailer.mu.Lock()
			defer mailer.mu.Unlock()
			writeJSON(w, 200, map[string]any{"email": mailer.email, "code": mailer.code, "challenge_id": mailer.challenge, "sent": mailer.sent, "fixture_only": true})
		case "/__fixture__/emotions":
			rows, queryErr := store.Pool.Query(r.Context(), "SELECT result FROM syntelle_conversation.operations WHERE kind='emotion-activation' AND status='received' ORDER BY id")
			if queryErr != nil {
				http.Error(w, "fixture query failed", 500)
				return
			}
			defer rows.Close()
			values := []json.RawMessage{}
			for rows.Next() {
				var v json.RawMessage
				if rows.Scan(&v) != nil {
					http.Error(w, "fixture scan failed", 500)
					return
				}
				values = append(values, v)
			}
			if rows.Err() != nil {
				http.Error(w, "fixture rows failed", 500)
				return
			}
			writeJSON(w, 200, values)
		case "/__fixture__/counts":
			writeJSON(w, 200, map[string]int32{"model": model.calls.Load(), "speech": voice.calls.Load(), "recognition": recognition.calls.Load()})
		case "/__fixture__/stop":
			stopOnce.Do(func() { close(stop) })
			w.WriteHeader(204)
		default:
			http.NotFound(w, r)
		}
	})
	address := "127.0.0.1:8083"
	// Explicit integration-only option for an unpublished container namespace.
	// Production handlers and default local fixture binding remain unchanged.
	if os.Getenv("SYNTELLE_STREAMING_FIXTURE_UNPUBLISHED_NETWORK") == "1" {
		address = "0.0.0.0:8083"
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: accountsService.Handler(service.Handler(control)), ReadHeaderTimeout: 5 * time.Second}
	finished := make(chan error, 1)
	go func() { finished <- server.Serve(listener) }()
	t.Log("OFFLINE FIXTURE READY http://127.0.0.1:8083; captured email and cached native audio only; zero real provider calls")
	select {
	case <-ctx.Done():
	case <-stop:
	}
	shutdown, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err = service.Shutdown(shutdown); err != nil {
		t.Error(err)
	}
	if err = conversations.Shutdown(shutdown); err != nil {
		t.Error(err)
	}
	if err = server.Shutdown(shutdown); err != nil {
		t.Error(err)
	}
	if err = <-finished; err != nil && !errors.Is(err, http.ErrServerClosed) {
		t.Error(err)
	}
}
