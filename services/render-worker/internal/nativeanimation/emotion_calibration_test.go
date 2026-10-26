package nativeanimation

import (
	"encoding/json"
	"testing"
)

func TestSourceBoundEmotionOpeningAndLegacyFallback(t *testing.T) {
	ref, _, _ := loadReference(t)
	for _, tc := range []struct {
		opening any
		valid   bool
	}{
		{nil, true}, {[]float64{0, 0, 0, 0, 0, 0, 0, .26, .4, 0}, true},
		{[]float64{0, .2}, false}, {[]float64{0, 0, 0, 0, 0, 0, 0, .6, .4, 0}, false},
	} {
		var ex map[string]any
		if err := json.Unmarshal(ref.Bundle.Expressions, &ex); err != nil {
			t.Fatal(err)
		}
		rig := map[string]any{"version": "emotion-rig/1", "validation": map[string]bool{"valid": true}}
		if tc.opening != nil {
			rig["rest_opening"] = tc.opening
		}
		ex["emotion_rig"] = rig
		raw, _ := json.Marshal(ex)
		c, err := Load(ref.Bundle.Motion, ref.Bundle.Gaze, raw)
		if (err == nil) != tc.valid {
			t.Fatalf("%v: %v", tc.opening, err)
		}
		if err == nil && tc.opening != nil && (c.EmotionRestOpening == nil || c.EmotionRestOpening[7] != .26) {
			t.Fatal("calibration discarded")
		}
		if tc.opening == nil && c.EmotionRestOpening != nil {
			t.Fatal("legacy package changed")
		}
	}
}
