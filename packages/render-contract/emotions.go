package rendercontract

import "math"

// EmotionStates is a fixed vocabulary, never model-supplied geometry.
var EmotionStates = [...]string{"neutral", "joyful", "playful", "affectionate", "curious", "sad", "angry", "fearful", "surprised", "disgusted"}

type EmotionBaseline struct {
	State     string  `json:"state"`
	Intensity float64 `json:"intensity"`
}

type EmotionTarget struct {
	State     string  `json:"state"`
	Intensity float64 `json:"intensity"`
	Energy    float64 `json:"energy"`
}

func EmotionIndex(state string) int {
	for i, name := range EmotionStates {
		if state == name {
			return i
		}
	}
	return -1
}

func UnitAmount(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v <= 1 }
func (v EmotionBaseline) Valid() bool {
	return (EmotionIndex(v.State) >= 0 || v.State == "" && v.Intensity == 0) && UnitAmount(v.Intensity)
}
func (v EmotionBaseline) Normalized() EmotionBaseline {
	if v.State == "" || v.State == "neutral" {
		return EmotionBaseline{State: "neutral"}
	}
	return v
}
func (v EmotionTarget) Valid() bool {
	return EmotionIndex(v.State) >= 0 && UnitAmount(v.Intensity) && UnitAmount(v.Energy)
}
func DirectedPerformance(version string) bool {
	return version == "performance-intent/2" || NaturalPerformance(version)
}

// EmotionWeights blend prepared displacement fields continuously, including
// transitions between different states. They never address an external resource.
type EmotionWeights [10]float64

type EmotionActivation struct {
	TurnID      string          `json:"turn_id"`
	PhraseIndex int             `json:"phrase_index"`
	Generation  uint64          `json:"generation"`
	Emotion     EmotionTarget   `json:"emotion"`
	Baseline    EmotionBaseline `json:"baseline"`
}
