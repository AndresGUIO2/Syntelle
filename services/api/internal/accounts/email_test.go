package accounts

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestResendPurposeCodeUsesStyledVerificationEmail(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, purpose, subject, heading string
	}{
		{"administration entry", "staff_entry", "Your Syntelle administration code", "Your administration code"},
		{"administration grant", "staff_grant", "Your Syntelle administration code", "Your administration code"},
		{"ownership transfer", "ownership_transfer", "Your Syntelle ownership code", "Your ownership verification code"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != http.MethodPost || r.URL.String() != "https://api.resend.com/emails" || r.Header.Get("Idempotency-Key") != "syntelle-privileged/fixture" {
					t.Fatal("incorrect privileged delivery contract")
				}
				var body struct {
					From, Subject, HTML, Text string
					To                        []string
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body.Subject != tc.subject || body.From != "Syntelle <login@example.test>" || len(body.To) != 1 || body.To[0] != "owner@example.test" {
					t.Fatal("incorrect message envelope")
				}
				for _, required := range []string{tc.heading, "123456", "role=\"presentation\"", "background-color: #1c1b20", "letter-spacing: 7px", "Valid for 10 minutes", "One-time use", "Keep this code private"} {
					if !strings.Contains(body.HTML, required) {
						t.Fatalf("styled verification message missing %q", required)
					}
				}
				for _, forbidden := range []string{"http://", "https://", "<script", "<img", "<iframe", "<link", "src=", "url(", "Your sign-in code", "help you sign in"} {
					if strings.Contains(body.HTML, forbidden) {
						t.Fatalf("unexpected content %q", forbidden)
					}
				}
				if !strings.Contains(body.Text, "123456") || !strings.Contains(body.Text, "10 minutes") || !strings.Contains(body.Text, "only be used once") {
					t.Fatal("missing plain-text alternative or expiry")
				}
				return jsonResponse(map[string]string{"id": "delivery-fixture"}, http.StatusOK), nil
			})}
			mailer := &ResendMailer{Key: "fixture-only", From: "Syntelle <login@example.test>", Client: client}
			if id, err := mailer.SendPurposeCode(context.Background(), "owner@example.test", "123456", "fixture", tc.purpose); err != nil || id != "delivery-fixture" || calls != 1 {
				t.Fatal("styled purpose delivery failed")
			}
			client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				return jsonResponse(map[string]string{"message": "rejected"}, http.StatusServiceUnavailable), nil
			})
			if _, err := mailer.SendPurposeCode(context.Background(), "owner@example.test", "123456", "fixture-error", tc.purpose); err == nil || calls != 2 {
				t.Fatal("delivery rejection was accepted or retried")
			}
		})
	}
}
