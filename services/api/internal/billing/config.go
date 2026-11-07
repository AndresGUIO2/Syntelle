// Package billing implements the sandbox-only paid workspace boundary.
package billing

import (
	"net/http"
	"os"
	"strings"
	"time"
)

const sandboxURL = "https://sandbox-api.paddle.com"

type Config struct {
	APIKey, WebhookSecret, ClientToken        string
	StarterPriceID, TeamPriceID, TopupPriceID string
	AuthorizeWorkspace                        func(http.ResponseWriter, *http.Request, string) (actorID, role string, suspended, ok bool)
	HTTPClient                                *http.Client
	// Now is only used for signature freshness; quota always uses PostgreSQL time.
	Now func() time.Time
}

func ConfigFromEnv() Config {
	return Config{APIKey: os.Getenv("PADDLE_API_KEY"), WebhookSecret: os.Getenv("PADDLE_WEBHOOK_SECRET"), ClientToken: os.Getenv("PADDLE_CLIENT_TOKEN"), StarterPriceID: os.Getenv("PADDLE_STARTER_PRICE_ID"), TeamPriceID: os.Getenv("PADDLE_TEAM_PRICE_ID"), TopupPriceID: os.Getenv("PADDLE_TOPUP_PRICE_ID")}
}
func (c Config) configured() bool {
	return strings.HasPrefix(c.APIKey, "pdl_sdbx_apikey_") && strings.HasPrefix(c.ClientToken, "test_") && c.WebhookSecret != "" && strings.HasPrefix(c.StarterPriceID, "pri_") && strings.HasPrefix(c.TeamPriceID, "pri_") && strings.HasPrefix(c.TopupPriceID, "pri_") && c.StarterPriceID != c.TeamPriceID && c.StarterPriceID != c.TopupPriceID && c.TeamPriceID != c.TopupPriceID
}

type Plan struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Amount            int    `json:"amount"`
	Currency          string `json:"currency"`
	Interval          string `json:"interval,omitempty"`
	IncludedMinutes   int    `json:"included_minutes,omitempty"`
	Minutes           int    `json:"minutes,omitempty"`
	Concurrency       int    `json:"concurrency,omitempty"`
	MaxSessionSeconds int    `json:"max_session_seconds,omitempty"`
}

var plans = []Plan{{ID: "test_starter", Name: "Test Starter", Amount: 1000, Currency: "USD", Interval: "month", IncludedMinutes: 60, Concurrency: 1, MaxSessionSeconds: 300}, {ID: "test_team", Name: "Test Team", Amount: 2000, Currency: "USD", Interval: "month", IncludedMinutes: 180, Concurrency: 2, MaxSessionSeconds: 300}}
var topup = Plan{ID: "test_topup", Name: "Test top-up", Amount: 500, Currency: "USD", Minutes: 30}

func planByID(id string) (Plan, bool) {
	for _, p := range plans {
		if p.ID == id {
			return p, true
		}
	}
	return Plan{}, false
}
func (c Config) price(id string) string {
	switch id {
	case "test_starter":
		return c.StarterPriceID
	case "test_team":
		return c.TeamPriceID
	case "test_topup":
		return c.TopupPriceID
	}
	return ""
}
func (c Config) plan(price string) string {
	if price == "" {
		return ""
	}
	switch price {
	case c.StarterPriceID:
		return "test_starter"
	case c.TeamPriceID:
		return "test_team"
	case c.TopupPriceID:
		return "test_topup"
	}
	return ""
}
