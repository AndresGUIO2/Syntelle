package streaming

import (
	"context"
	"io"
	"net/http"
	"strings"
	"syntelle/api/internal/accounts"
	"testing"
)

type widgetRoundTrip func(*http.Request) (*http.Response, error)

func (f widgetRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestTurnstileVerifierRejectsWrongBindings(t *testing.T) {
	for _, body := range []string{`{"success":false}`, `{"success":true,"hostname":"evil.example","action":"widget_start","cdata":"nonce"}`, `{"success":true,"hostname":"app.syntelle.online","action":"other","cdata":"nonce"}`, `{"success":true,"hostname":"app.syntelle.online","action":"widget_start","cdata":"other"}`} {
		client := &http.Client{Transport: widgetRoundTrip(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
		})}
		if NewTurnstileVerifier("private", "app.syntelle.online", client)(context.Background(), "token", "127.0.0.1", "nonce") == nil {
			t.Fatal("accepted mismatched challenge", body)
		}
	}
	client := &http.Client{Transport: widgetRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://challenges.cloudflare.com/turnstile/v0/siteverify" {
			t.Fatal("unexpected verification endpoint")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"success":true,"hostname":"app.syntelle.online","action":"widget_start","cdata":"nonce"}`))}, nil
	})}
	if err := NewTurnstileVerifier("private", "app.syntelle.online", client)(context.Background(), "token", "127.0.0.1", "nonce"); err != nil {
		t.Fatal(err)
	}
}
func TestIntegrationCommandsCannotChangePrivateConfiguration(t *testing.T) {
	s := &Service{}
	session := &streamSession{capabilities: map[string]bool{"speak": true}}
	for _, command := range []Command{{Type: "configure"}, {Type: "preview"}, {Type: "message"}, {Type: "speak", Settings: &accounts.Settings{}}} {
		if _, err := s.command(context.Background(), session, command); err == nil {
			t.Fatal("unauthorized command accepted", command.Type)
		}
	}
}
