package rendercontract

import (
	"math"
	"testing"
)

func TestEmotionVocabularyAndBounds(t *testing.T) {
	for _, state := range EmotionStates {
		t.Run(state, func(t *testing.T) {
			v := Defaults()
			v.EmotionBaseline = EmotionBaseline{State: state, Intensity: .7}
			if err := v.Validate(); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, v := range []EmotionTarget{{State: "invented"}, {State: "sad", Intensity: math.NaN()}, {State: "joyful", Intensity: 1.1}, {State: "angry", Energy: -.1}} {
		if v.Valid() {
			t.Fatal("invalid emotional direction admitted", v)
		}
	}
	if !(EmotionBaseline{}).Valid() || (EmotionBaseline{}).Normalized().State != "neutral" {
		t.Fatal("legacy settings lost neutral compatibility")
	}
}
