package nativeanimation

import (
	"crypto/sha256"
	"encoding/hex"
	"syntelle/render-contract"
	"testing"
)

func TestInvalidLaughterSpanFailsBeforePreparation(t *testing.T) {
	samples := make([]int16, 480)
	hash := sha256.Sum256(make([]byte, 960))
	valid := rendercontract.LaughterSpan{Version: "laughter-span/1", Count: 480, PCMHash: hex.EncodeToString(hash[:]), RecordingHash: hex.EncodeToString(hash[:])}
	if err := ValidateLaughterSpan(&valid, samples, 24000); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*rendercontract.LaughterSpan){func(s *rendercontract.LaughterSpan) { s.Offset = -1 }, func(s *rendercontract.LaughterSpan) { s.Count = 481 }, func(s *rendercontract.LaughterSpan) { s.PCMHash = "bad" }, func(s *rendercontract.LaughterSpan) { s.Version = "future" }} {
		s := valid
		change(&s)
		if err := ValidateLaughterSpan(&s, samples, 24000); err == nil {
			t.Fatal("invalid binding admitted", s)
		}
	}
}
