package conversation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestProfileRejectsUnpreparedCapabilitiesAndUnboundedLaughter(t *testing.T) {
	hash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	p := PerformanceProfile{Version: "character-performance/1", PackageHash: hash, Reviewed: true, Capabilities: []string{"nod"}}
	path := filepath.Join(t.TempDir(), "profile.json")
	write := func() {
		data, _ := json.Marshal(p)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write()
	if _, err := LoadPerformanceProfile(path, hash); err != nil {
		t.Fatal(err)
	}
	p.Capabilities = []string{"execute"}
	write()
	if _, err := LoadPerformanceProfile(path, hash); err == nil {
		t.Fatal("unprepared capability accepted")
	}
	p.Capabilities = []string{"laugh"}
	raw, _ := json.Marshal(cachedSpeech{Audio: performanceFixtureWAV(), Voice: "voice", Language: "en-US"})
	p.Laughter = map[string]json.RawMessage{"voice|en-US": raw}
	write()
	if _, err := LoadPerformanceProfile(path, hash); err == nil {
		t.Fatal("unbounded cached laughter accepted")
	}
}

func TestPreparedPrivateProfileLoads(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", ".tools", "expressive-performance-67", "profile-reviewed.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Skip("private prepared profile unavailable")
	}
	if err != nil {
		t.Fatal(err)
	}
	var p PerformanceProfile
	if err = json.Unmarshal(data, &p); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadPerformanceProfile(path, p.PackageHash); err != nil {
		t.Fatal(err)
	}
}
