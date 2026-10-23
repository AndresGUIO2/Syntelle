package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"syntelle/api/internal/conversation"
	"syntelle/api/internal/speech"
)

type providerFixture struct{ calls int }

func (p *providerFixture) Synthesize(context.Context, speech.Input) (speech.Result, error) {
	p.calls++
	return speech.Result{Audio: make([]byte, 44), DurationMS: 1000, Visemes: []speech.Viseme{{ID: 0}, {ID: 2, OffsetMS: 100}}}, nil
}

type ledgerFixture struct{ reservations, finishes int }

func (l *ledgerFixture) Reserve(context.Context, speech.Input, string, int) (int64, *speech.Result, error) {
	l.reservations++
	return 1, nil, nil
}
func (l *ledgerFixture) Finish(context.Context, int64, *speech.Result, string, bool) error {
	l.finishes++
	return nil
}

func TestSpeechAccessGuardInOwnerAndTrialModes(t *testing.T) {
	for _, owner := range []bool{false, true} {
		mode := "trial"
		if owner {
			mode = "owner"
		}
		t.Run(mode, func(t *testing.T) {
			for _, tc := range []struct {
				name, method, host, origin string
				want                       int
				paid                       bool
			}{
				{"allowed simple POST", "POST", "127.0.0.1:8080", "http://127.0.0.1:5173", 200, true},
				{"allowed localhost", "POST", "localhost:8080", "http://localhost:5173", 200, true},
				{"foreign simple POST", "POST", "127.0.0.1:8080", "https://unrelated.example", 403, false},
				{"missing origin", "POST", "127.0.0.1:8080", "", 403, false},
				{"invalid host", "POST", "unrelated.example", "http://127.0.0.1:5173", 403, false},
				{"read-only status", "GET", "127.0.0.1:8080", "", 200, false},
				{"foreign status", "GET", "127.0.0.1:8080", "https://unrelated.example", 403, false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					provider, ledger := &providerFixture{}, &ledgerFixture{}
					voice := speech.NewService(provider, ledger, "canadacentral")
					var service *conversation.Service
					if owner {
						service = conversation.New(nil, nil, nil, nil)
					}
					handler := newApplicationHandler("", http.DefaultClient, voice, service)
					body, err := json.Marshal(speech.Input{Text: "Hello.", Language: "en-US", Voice: "en-US-Ava:DragonHDLatestNeural", RequestKey: "fixture-request-01"})
					if err != nil {
						t.Fatal(err)
					}
					path := "/api/v1/speech/synthesize"
					if tc.method == "GET" {
						path = "/api/v1/speech/status"
					}
					r := httptest.NewRequest(tc.method, "http://"+tc.host+path, bytes.NewReader(body))
					r.Header.Set("Origin", tc.origin)
					r.Header.Set("Content-Type", "text/plain")
					w := httptest.NewRecorder()
					handler.ServeHTTP(w, r)
					wantCalls := 0
					if tc.paid {
						wantCalls = 1
					}
					if w.Code != tc.want || provider.calls != wantCalls || ledger.reservations != wantCalls || ledger.finishes != wantCalls {
						t.Fatalf("status=%d provider=%d reservations=%d finishes=%d", w.Code, provider.calls, ledger.reservations, ledger.finishes)
					}
				})
			}
		})
	}
}

func TestApplicationPreservesShutdownAndDisabledSpeech(t *testing.T) {
	service := conversation.New(nil, nil, nil, nil)
	handler := newApplicationHandler("", http.DefaultClient, nil, service)
	shutdown, ok := handler.(interface{ Shutdown(context.Context) error })
	if !ok {
		t.Fatal("HTTP guard hid upgraded-connection shutdown")
	}
	if err := shutdown.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "http://127.0.0.1:8080/api/v1/speech/synthesize", nil)
	r.Header.Set("Origin", "http://127.0.0.1:5173")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
}
