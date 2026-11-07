package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func fixtureConfig() Config {
	return Config{APIKey: "pdl_sdbx_apikey_fixture", ClientToken: "test_fixture", WebhookSecret: "fixture-signing-secret", StarterPriceID: "pri_starter", TeamPriceID: "pri_team", TopupPriceID: "pri_topup"}
}
func signed(raw []byte, at time.Time, secret string) string {
	ts := fmt.Sprint(at.Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + ":"))
	mac.Write(raw)
	return "ts=" + ts + ";h1=" + hex.EncodeToString(mac.Sum(nil))
}
func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func TestSignatureAuthenticatesExactBytesAndFreshness(t *testing.T) {
	now := time.Unix(1780000000, 0)
	raw := []byte(`{"event_id":"evt_fixture"}`)
	secret := "fixture-secret"
	good := signed(raw, now, secret)
	for _, tc := range []struct {
		name   string
		raw    []byte
		header string
		now    time.Time
		want   bool
	}{
		{"valid", raw, good, now, true}, {"previous secret alongside new", raw, good + ";h1=00", now, true},
		{"tampered", []byte(`{"event_id":"evt_other"}`), good, now, false}, {"whitespace matters", append(raw, ' '), good, now, false},
		{"stale", raw, good, now.Add(6 * time.Second), false}, {"future", raw, good, now.Add(-6 * time.Second), false},
		{"duplicate timestamp", raw, good + ";ts=1780000000", now, false}, {"missing signature", raw, "ts=1780000000", now, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := verifySignature(tc.raw, tc.header, secret, tc.now); got != tc.want {
				t.Fatalf("signature=%v want %v", got, tc.want)
			}
		})
	}
}

func TestSandboxConfigurationFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Config)
		want   bool
	}{
		{"fixture", func(*Config) {}, true}, {"production key", func(c *Config) { c.APIKey = "pdl_live_apikey_fixture" }, false},
		{"live client token", func(c *Config) { c.ClientToken = "live_fixture" }, false}, {"missing signing secret", func(c *Config) { c.WebhookSecret = "" }, false},
		{"duplicate price", func(c *Config) { c.TopupPriceID = c.StarterPriceID }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := fixtureConfig()
			tc.change(&c)
			if c.configured() != tc.want {
				t.Fatalf("configured=%v", c.configured())
			}
		})
	}
}

func TestProviderUsesOnlySandboxAndDoesNotFollowRedirects(t *testing.T) {
	c := fixtureConfig()
	calls := 0
	c.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Scheme != "https" || r.URL.Host != "sandbox-api.paddle.com" {
			t.Fatalf("unexpected destination %s", r.URL)
		}
		if r.Header.Get("Authorization") != "Bearer "+c.APIKey {
			t.Fatal("missing credential")
		}
		out := response(302, `secret provider body`)
		out.Header.Set("Location", "https://untrusted.example/steal")
		return out, nil
	})}
	s := New(nil, c)
	_, err := s.provider(context.Background(), http.MethodGet, "/prices/pri_starter", nil)
	if err == nil || strings.Contains(err.Error(), "secret") || calls != 1 {
		t.Fatalf("calls=%d error=%v", calls, err)
	}
}

func TestCatalogVerificationAndCancellation(t *testing.T) {
	for _, tc := range []struct {
		name, plan, body string
		pass             bool
	}{
		{"monthly", "test_starter", `{"id":"pri_starter","status":"active","unit_price":{"amount":"1000","currency_code":"USD"},"billing_cycle":{"interval":"month","frequency":1}}`, true},
		{"wrong amount", "test_starter", `{"id":"pri_starter","status":"active","unit_price":{"amount":"999","currency_code":"USD"},"billing_cycle":{"interval":"month","frequency":1}}`, false},
		{"wrong period", "test_starter", `{"id":"pri_starter","status":"active","unit_price":{"amount":"1000","currency_code":"USD"},"billing_cycle":{"interval":"year","frequency":1}}`, false},
		{"one time", "test_topup", `{"id":"pri_topup","status":"active","unit_price":{"amount":"500","currency_code":"USD"},"billing_cycle":null}`, true},
		{"recurring topup", "test_topup", `{"id":"pri_topup","status":"active","unit_price":{"amount":"500","currency_code":"USD"},"billing_cycle":{"interval":"month","frequency":1}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := fixtureConfig()
			c.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) { return response(200, `{"data":`+tc.body+`}`), nil })}
			err := New(nil, c).verifyPrice(context.Background(), tc.plan)
			if (err == nil) != tc.pass {
				t.Fatalf("err=%v", err)
			}
		})
	}
	c := fixtureConfig()
	c.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New(nil, c).provider(ctx, http.MethodGet, "/prices/pri_starter", nil); err == nil {
		t.Fatal("canceled request succeeded")
	}
}
