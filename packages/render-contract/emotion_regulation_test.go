package rendercontract

import (
	"encoding/json"
	"testing"
)

func TestEmotionRegulationDefaultsAndExplicitDisable(t *testing.T) {
	for _, tt := range []struct {
		name, wire string
		enabled    bool
	}{
		{"missing", `{}`, true}, {"disabled", `{"emotion_regulation":{"enabled":false,"temperament":"warm"}}`, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var settings Settings
			if err := json.Unmarshal([]byte(tt.wire), &settings); err != nil {
				t.Fatal(err)
			}
			if settings.EmotionRegulation.Normalized().Enabled != tt.enabled {
				t.Fatal(settings)
			}
			data, err := json.Marshal(settings)
			if err != nil {
				t.Fatal(err)
			}
			var restored Settings
			if err = json.Unmarshal(data, &restored); err != nil {
				t.Fatal(err)
			}
			if restored.EmotionRegulation.Enabled != tt.enabled {
				t.Fatal(string(data))
			}
		})
	}
	for _, wire := range []string{`{"enabled":true,"temperament":"villain"}`, `{"enabled":true,"multiplier":100}`, `{"temperament":"warm"}`, `null`} {
		var r EmotionRegulation
		if json.Unmarshal([]byte(wire), &r) == nil {
			t.Fatal("untrusted policy definition accepted", wire)
		}
	}
}
