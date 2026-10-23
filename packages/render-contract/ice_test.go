package rendercontract

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"strings"
	"testing"
)

func TestTURNRESTCredentials(t *testing.T) {
	t.Setenv("SYNTELLE_DEPLOYMENT_MODE", "cloud")
	servers, err := ICEServers()
	if err != nil || len(servers) != 0 {
		t.Fatalf("TURN may be deferred: servers=%v err=%v", servers, err)
	}
	t.Setenv("SYNTELLE_TURN_SECRET", strings.Repeat("x", 32))
	t.Setenv("SYNTELLE_TURN_URLS", "turns:relay.example:443?transport=tcp")
	servers, err = ICEServers()
	if err != nil {
		t.Fatal(err)
	}
	mac := hmac.New(sha1.New, []byte(strings.Repeat("x", 32)))
	mac.Write([]byte(servers[0].Username))
	if servers[0].Credential != base64.StdEncoding.EncodeToString(mac.Sum(nil)) {
		t.Fatal("invalid expiring credential")
	}
	t.Setenv("SYNTELLE_STUN_URLS", "stun:discovery.example:3478")
	servers, err = ICEServers()
	if err != nil || len(servers) != 2 || servers[1].Credential != "" {
		t.Fatal("optional STUN settings must not contain TURN credentials")
	}
	t.Setenv("SYNTELLE_TURN_URLS", "")
	t.Setenv("SYNTELLE_TURN_SECRET", "")
	servers, err = ICEServers()
	if err != nil || len(servers) != 1 || servers[0].URLs[0] != "stun:discovery.example:3478" {
		t.Fatal("STUN-only cloud configuration must be accepted")
	}
	t.Setenv("SYNTELLE_TURN_URLS", "turns:relay.example:443?transport=tcp")
	if _, err := ICEServers(); err == nil {
		t.Fatal("TURN URL without a signing secret accepted")
	}
	t.Setenv("SYNTELLE_TURN_URLS", "https://evil.invalid")
	t.Setenv("SYNTELLE_TURN_SECRET", strings.Repeat("x", 32))
	if _, err := ICEServers(); err == nil {
		t.Fatal("non TURN endpoint accepted")
	}
}
