package conversation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLaughterPreviewReusesReviewedBytesWithoutProvidersOrHistory(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", ".tools", "expressive-performance-67", "profile-reviewed.json")
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Skip("private reviewed fixture unavailable")
	}
	if err != nil {
		t.Fatal(err)
	}
	var p PerformanceProfile
	if err = json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	s := &Service{Performance: &p} // No model, speech service or store exists here.
	for i := 0; i < 2; i++ {
		result, err := s.LaughterPreview("en-US-Ava:DragonHDLatestNeural", "en-US")
		if err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(result)
		var got cachedSpeech
		_ = json.Unmarshal(encoded, &got)
		var original cachedSpeech
		_ = json.Unmarshal(p.Laughter["en-US-Ava:DragonHDLatestNeural|en-US"], &original)
		if string(got.Audio) != string(original.Audio) || got.Duration != original.Duration {
			t.Fatal("preview altered accepted audio")
		}
	}
	if len(s.laughPolicy) != 0 {
		t.Fatal("preview consumed conversational laughter allowance")
	}
	if _, err = s.LaughterPreview("other-voice", "en-US"); err == nil {
		t.Fatal("preview silently substituted voice")
	}
}
