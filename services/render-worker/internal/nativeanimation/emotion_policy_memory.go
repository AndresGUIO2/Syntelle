package nativeanimation

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"syntelle/render-contract"
)

type moodMemory struct {
	Version                  string
	Motion                   EmotionSnapshot
	Sustained                rendercontract.EmotionTarget
	Baseline                 rendercontract.EmotionBaseline
	Regulation               rendercontract.EmotionRegulation
	Automatic, Reduced       bool
	LastActivity, LastChange float64
	Changes                  []float64
	Confirmation             string
	ConfirmationAt           float64
}

func (c EmotionController) MoodMemory(ms float64) json.RawMessage {
	motion := c.Motion
	motion.Start -= ms
	changes := append([]float64(nil), c.changes[:c.changeCount]...)
	for i := range changes {
		changes[i] -= ms
	}
	data, _ := json.Marshal(moodMemory{Version: "committed-mood/1", Motion: motion, Sustained: c.sustained, Baseline: c.key.Base, Regulation: c.key.Regulation, Automatic: c.key.Automatic, Reduced: c.key.Reduced, LastActivity: c.lastActivity - ms, LastChange: c.lastChange - ms, Changes: changes, Confirmation: c.confirmation, ConfirmationAt: c.confirmationAt - ms})
	return data
}

func (c *EmotionController) RestoreMood(data json.RawMessage) error {
	if !c.Persistent || len(data) > 16384 {
		return errors.New("mood restoration not supported")
	}
	var m moodMemory
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&m) != nil || m.Version != "committed-mood/1" || !m.Motion.Effective.Valid() || !m.Sustained.Valid() || m.Baseline != c.key.Base || m.Regulation != c.key.Regulation || m.Automatic != c.key.Automatic || m.Reduced != c.key.Reduced || len(m.Changes) > 2 {
		return errors.New("mood settings or identity mismatch")
	}
	for _, values := range []rendercontract.EmotionWeights{m.Motion.From, m.Motion.To} {
		sum := 0.
		for _, v := range values {
			if !rendercontract.UnitAmount(v) {
				return errors.New("invalid mood weight")
			}
			sum += v
		}
		if sum > 1.000001 {
			return errors.New("invalid blended mood")
		}
	}
	times := append([]float64{m.Motion.Start, m.Motion.Duration, m.LastActivity, m.LastChange, m.ConfirmationAt}, m.Changes...)
	for _, v := range times {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return errors.New("invalid mood clock")
		}
	}
	if m.Motion.Start > 0 || m.Motion.Duration < 0 || m.Motion.Duration > 60000 || m.LastActivity > 0 {
		return errors.New("invalid mood age")
	}
	switch m.Motion.Phase {
	case "baseline", "reaction", "holding", "recovery", "bridge":
	default:
		return errors.New("invalid mood phase")
	}
	c.Motion = m.Motion
	c.sustained = m.Sustained
	c.lastActivity = m.LastActivity
	c.lastChange = m.LastChange
	c.changeCount = len(m.Changes)
	copy(c.changes[:], m.Changes)
	c.confirmation = m.Confirmation
	c.confirmationAt = m.ConfirmationAt
	c.releasing = false
	c.initialized = true
	return nil
}
