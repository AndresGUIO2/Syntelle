package conversation

import (
	"bytes"
	"encoding/json"
	"syntelle/render-contract"
)

func validEmotion(v *rendercontract.EmotionTarget) *rendercontract.EmotionTarget {
	if v == nil || !v.Valid() {
		return nil
	}
	copy := *v
	return &copy
}

func decodeEmotion(raw json.RawMessage) *rendercontract.EmotionTarget {
	var v struct {
		State     string   `json:"state"`
		Intensity *float64 `json:"intensity"`
		Energy    *float64 `json:"energy"`
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&v) != nil || v.Intensity == nil || v.Energy == nil {
		return nil
	}
	return validEmotion(&rendercontract.EmotionTarget{State: v.State, Intensity: *v.Intensity, Energy: *v.Energy})
}

func phraseVersion(v string) string {
	if v == "performance-intent/3" {
		return v
	}
	return "performance-intent/2"
}

func updateExpressionBaseline(before, next ExpressionBaseline) ExpressionBaseline {
	next.LastEmotion = nil
	next.EmotionPhase = ""
	if before.Emotion.Normalized() == next.Emotion.Normalized() && before.Regulation.Normalized() == next.Regulation.Normalized() {
		next.LastEmotion = before.LastEmotion
		next.EmotionPhase = before.EmotionPhase
	}
	return next
}
