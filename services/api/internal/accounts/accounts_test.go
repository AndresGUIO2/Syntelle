package accounts

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonResponse(body any, status int) *http.Response {
	b, _ := json.Marshal(body)
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewReader(b))}
}

func TestSettingsAndEmailValidation(t *testing.T) {
	t.Parallel()
	if err := Defaults().Validate(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		edit func(*Settings)
	}{
		{"legacy character", func(s *Settings) { s.Character = "cafe" }},
		{"unknown voice", func(s *Settings) { s.Voice = "unknown" }},
		{"foreign background", func(s *Settings) { s.Background = "https://example.invalid/image" }},
		{"excessive blur", func(s *Settings) { s.Blur = 21 }},
		{"invalid expression", func(s *Settings) { s.Smile = -1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := Defaults()
			tc.edit(&s)
			if s.Validate() == nil {
				t.Fatal("invalid setting accepted")
			}
		})
	}
	for _, tc := range []struct {
		input string
		ok    bool
	}{
		{" owner@example.com ", true}, {"Owner <owner@example.com>", false}, {"owner@example.com\r\nBcc: other@example.com", false}, {"missing-at", false},
	} {
		t.Run(tc.input, func(t *testing.T) {
			_, err := normalizeEmail(tc.input)
			if (err == nil) != tc.ok {
				t.Fatal("unexpected email validation")
			}
		})
	}
}

func TestLocalAccessAndCookieSecurity(t *testing.T) {
	t.Parallel()
	s := &Service{cfg: Config{Origin: "http://127.0.0.1:5173", Now: time.Now}}
	for _, tc := range []struct {
		name, method, origin, header, authorization, host string
		ok                                                bool
	}{
		{"navigation", "GET", "", "", "", "127.0.0.1:5173", true},
		{"browser write", "POST", "http://127.0.0.1:5173", "1", "", "127.0.0.1:5173", true},
		{"missing csrf", "POST", "http://127.0.0.1:5173", "", "", "127.0.0.1:5173", false},
		{"cross site", "POST", "https://foreign.invalid", "1", "", "127.0.0.1:5173", false},
		{"integration", "POST", "", "", "Bearer fixture", "127.0.0.1:8080", true},
		{"foreign integration origin", "POST", "https://foreign.invalid", "", "Bearer fixture", "127.0.0.1:8080", false},
		{"public host", "GET", "", "", "", "public.invalid", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "http://"+tc.host+"/", nil)
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("X-Syntelle-Request", tc.header)
			r.Header.Set("Authorization", tc.authorization)
			if s.AllowedRequest(r) != tc.ok {
				t.Fatal("incorrect request boundary")
			}
		})
	}
	w := httptest.NewRecorder()
	s.cookie(w, "fixture", time.Now().Add(time.Hour))
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode || cookies[0].Secure {
		t.Fatal("incorrect loopback cookie")
	}
	s.cfg.Origin = "https://syntelle.invalid"
	if !s.secureCookie() {
		t.Fatal("hosted cookie is not secure")
	}
}

func TestEncryptedOAuthStateAndReturn(t *testing.T) {
	t.Parallel()
	s := &Service{cfg: Config{Secret: strings.Repeat("x", 32)}}
	data := []byte(`{"verifier":"private"}`)
	ciphertext, err := s.encrypt(data)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ciphertext, data) {
		t.Fatal("flow is stored as plaintext")
	}
	plain, err := s.decrypt(ciphertext)
	if err != nil || !bytes.Equal(plain, data) {
		t.Fatal("flow restoration failed")
	}
	ciphertext[len(ciphertext)-1] ^= 1
	if _, err = s.decrypt(ciphertext); err == nil {
		t.Fatal("tampered flow accepted")
	}
	if safeReturn("//foreign.invalid") != "/" || safeReturn("https://foreign.invalid") != "/" {
		t.Fatal("external redirect allowed")
	}
	if safeReturn("/?view=stream") != "/?view=stream" {
		t.Fatal("stream workspace return lost")
	}
	if same(s.verifier("a", "bc"), s.verifier("ab", "c")) {
		t.Fatal("ambiguous verifier binding")
	}
}

func TestResendDeliveryUsesOneSelfContainedRequest(t *testing.T) {
	t.Parallel()
	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != "https://api.resend.com/emails" || r.Header.Get("Idempotency-Key") != "syntelle-login/fixture" {
			t.Fatal("incorrect delivery endpoint or idempotency")
		}
		var body map[string]any
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			t.Fatal("invalid email payload")
		}
		html, ok := body["html"].(string)
		if !ok || !strings.Contains(html, "123456") || !strings.Contains(html, "Your sign-in code") {
			t.Fatal("styled email missing its code")
		}
		for _, forbidden := range []string{"http://", "https://", "<script", "<img", "<iframe", "<link", "src=", "url("} {
			if strings.Contains(strings.ToLower(html), forbidden) {
				t.Fatal("remote resource or active content introduced")
			}
		}
		if !strings.Contains(body["text"].(string), "123456") || strings.Contains(body["text"].(string), "http") {
			t.Fatal("incorrect plain-text code")
		}
		return jsonResponse(map[string]string{"id": "delivery-fixture"}, 200), nil
	})}
	m := &ResendMailer{Key: "fixture-only", From: "Syntelle <login@example.com>", Client: client}
	if id, err := m.SendCode(context.Background(), "owner@example.com", "123456", "fixture"); err != nil || id != "delivery-fixture" || calls != 1 {
		t.Fatal("delivery failed")
	}
	client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return jsonResponse(map[string]string{"message": "rejected"}, 503), nil
	})
	if _, err := m.SendCode(context.Background(), "owner@example.com", "123456", "fixture-error"); err == nil || calls != 2 {
		t.Fatal("delivery error was retried or accepted")
	}
}

func TestSignInTemplateEscapesDynamicContent(t *testing.T) {
	t.Parallel()
	html, _, err := renderSignInEmail(`<img src=x onerror=alert(1)>`)
	if err != nil || strings.Contains(html, "<img") || !strings.Contains(html, "&lt;img") {
		t.Fatal("dynamic content was not escaped")
	}
}

func TestGoogleOIDCValidatesNonceAudienceAndSignature(t *testing.T) {
	t.Parallel()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	nonce := "expected-nonce"
	encode := base64.RawURLEncoding.EncodeToString
	sign := func(audience, tokenNonce string) string {
		h, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "fixture"})
		p, _ := json.Marshal(map[string]any{"iss": "https://accounts.google.com", "aud": audience, "sub": "stable-subject", "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Add(-time.Minute).Unix(), "nonce": tokenNonce, "email": "owner@example.com", "email_verified": true, "name": "Owner"})
		unsigned := encode(h) + "." + encode(p)
		sum := sha256.Sum256([]byte(unsigned))
		signature, e := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
		if e != nil {
			t.Fatal(e)
		}
		return unsigned + "." + encode(signature)
	}
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			return jsonResponse(map[string]any{"issuer": "https://accounts.google.com", "authorization_endpoint": "https://accounts.google.com/o/oauth2/v2/auth", "token_endpoint": "https://oauth2.googleapis.com/token", "jwks_uri": "https://accounts.google.com/fixture-jwks", "id_token_signing_alg_values_supported": []string{"RS256"}}, 200), nil
		case "/fixture-jwks":
			return jsonResponse(map[string]any{"keys": []any{map[string]string{"kty": "RSA", "kid": "fixture", "alg": "RS256", "use": "sig", "n": encode(key.N.Bytes()), "e": encode(big.NewInt(int64(key.E)).Bytes())}}}, 200), nil
		default:
			return nil, fmt.Errorf("unexpected fixture request")
		}
	})}
	s := &Service{cfg: Config{HTTPClient: client}}
	ctx := oidcTestContext(client)
	for _, tc := range []struct {
		name, audience, nonce string
		valid                 bool
	}{
		{"valid", "client", nonce, true}, {"nonce mismatch", "client", "wrong", false}, {"audience mismatch", "foreign", nonce, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token := (&oauth2.Token{AccessToken: "fixture"}).WithExtra(map[string]any{"id_token": sign(tc.audience, tc.nonce)})
			identity, err := s.socialIdentity(ctx, "google", "client", nonce, token)
			if (err == nil) != tc.valid {
				t.Fatal("incorrect identity validation")
			}
			if tc.valid && (identity.Subject != "stable-subject" || !identity.Verified) {
				t.Fatal("verified identity was lost")
			}
		})
	}
}

func oidcTestContext(client *http.Client) context.Context {
	return context.WithValue(context.Background(), oauth2.HTTPClient, client)
}
