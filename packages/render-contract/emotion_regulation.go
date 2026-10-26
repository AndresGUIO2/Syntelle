package rendercontract

import (
	"bytes"
	"encoding/json"
	"errors"
)

const EmotionPolicyVersion = "emotion-regulation/1"

type EmotionRegulation struct {
	Enabled     bool   `json:"enabled"`
	Temperament string `json:"temperament"`
}

func (v EmotionRegulation) Normalized() EmotionRegulation {
	if v.Temperament == "" {
		return EmotionRegulation{true, "calm"}
	}
	return v
}
func (v EmotionRegulation) Valid() bool {
	v = v.Normalized()
	return v.Temperament == "calm" || v.Temperament == "warm" || v.Temperament == "expressive"
}
func (v *EmotionRegulation) UnmarshalJSON(data []byte) error {
	var wire struct {
		Enabled     *bool  `json:"enabled"`
		Temperament string `json:"temperament"`
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if d.Decode(&wire) != nil || wire.Enabled == nil {
		return errors.New("choose emotion stability and temperament")
	}
	if wire.Temperament == "" {
		wire.Temperament = "calm"
	}
	*v = EmotionRegulation{*wire.Enabled, wire.Temperament}
	if !v.Valid() {
		return errors.New("choose a supported temperament")
	}
	return nil
}
func (v EmotionRegulation) MarshalJSON() ([]byte, error) {
	type wire EmotionRegulation
	return json.Marshal(wire(v.Normalized()))
}

type EmotionTemperament struct {
	Multiplier float64 `json:"multiplier"`
	DwellMS    float64 `json:"dwell_ms"`
	RecoveryMS float64 `json:"recovery_ms"`
}

func (v EmotionRegulation) Profile() EmotionTemperament {
	switch v.Normalized().Temperament {
	case "warm":
		return EmotionTemperament{.8, 4500, 6000}
	case "expressive":
		return EmotionTemperament{1, 3000, 8000}
	default:
		return EmotionTemperament{.6, 6000, 4000}
	}
}
func EmotionCeiling(state string) float64 {
	values := [10]float64{0, .65, .55, .45, .5, .45, .3, .35, .5, .3}
	if i := EmotionIndex(state); i >= 0 {
		return values[i]
	}
	return 0
}

// Private worker evidence. Browser conversation JSON must never populate it.
type EmotionDecision struct {
	Instance string `json:"instance"`

	Version        string            `json:"version"`
	PolicyRevision uint64            `json:"policy_revision"`
	Sequence       uint64            `json:"sequence"`
	TurnID         string            `json:"turn_id"`
	PhraseIndex    int               `json:"phrase_index"`
	Generation     uint64            `json:"generation"`
	Baseline       EmotionBaseline   `json:"baseline"`
	Regulation     EmotionRegulation `json:"regulation"`
	Proposed       *EmotionTarget    `json:"proposed,omitempty"`
	Effective      EmotionTarget     `json:"effective"`
	Phase          string            `json:"phase"`
	Reason         string            `json:"reason"`
}
