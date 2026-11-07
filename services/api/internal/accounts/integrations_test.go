package accounts

import "testing"

func TestWidgetOriginsAndCapabilities(t *testing.T) {
	for _, raw := range []string{"https://*.example.com", "https://example.com/path", "https://user:password@example.com", "http://example.com", "null", "https://example.com?x=1", "https://example.com#x"} {
		if _, err := NormalizeWidgetOrigin(raw); err == nil {
			t.Errorf("accepted invalid origin %q", raw)
		}
	}
	got, err := NormalizeWidgetOrigin("https://EXAMPLE.com:443/")
	if err != nil || got != "https://example.com" {
		t.Fatal(got, err)
	}
	c := IntegrationConfig{Settings: Defaults(), AllowedOrigins: []string{"https://example.com", "https://example.com/"}}
	if err := c.normalize(); err != nil || len(c.AllowedOrigins) != 1 || c.DailyLimitSeconds != 3600 || c.Appearance.Position != "bottom-right" {
		t.Fatal(c, err)
	}
	c.Capabilities = []string{"configure"}
	if c.normalize() == nil {
		t.Fatal("visitor may configure private instructions")
	}
	c.Capabilities = []string{"message"}
	c.DailyLimitSeconds = 86401
	if c.normalize() == nil {
		t.Fatal("accepted unbounded daily allowance")
	}
}
