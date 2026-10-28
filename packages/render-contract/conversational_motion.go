package rendercontract

import "math"

const ConversationalMotionVersion = "conversational-motion/1"

// MotionTiming is private application-owned timing, never LLM direction.
type MotionTiming struct {
	Version string  `json:"version"`
	ReplyID string  `json:"reply_id"`
	Offset  float64 `json:"offset_seconds"`
}

func (m *MotionTiming) Valid() bool {
	return m != nil && m.Version == ConversationalMotionVersion && m.ReplyID != "" && !math.IsNaN(m.Offset) && !math.IsInf(m.Offset, 0) && m.Offset >= 0
}

// GestureWindow is owned sequentially by one reply/scene. Each repeated cycle
// reserves credit. Repeated sequences conservatively retain their credits until
// 20 seconds after the sequence ends, covering native timing variation.
type GestureWindow struct {
	starts      []float64
	last, end   float64
	initialized bool
}

func (w *GestureWindow) Admit(at, duration float64, repetitions int) bool {
	if math.IsNaN(at) || math.IsInf(at, 0) || at < 0 || math.IsNaN(duration) || math.IsInf(duration, 0) || duration <= 0 || repetitions < 1 || repetitions > 8 {
		return false
	}
	if w.initialized && (at-w.last < 2.2-1e-9 || at < w.end-1e-9) {
		return false
	}
	kept := make([]float64, 0, 8)
	for _, start := range w.starts {
		if start > at-20 {
			kept = append(kept, start)
		}
	}
	for i := 0; i < repetitions; i++ {
		t := at + duration*float64(i)/float64(repetitions)
		if repetitions > 1 {
			t = at + duration
		}
		count := 0
		for _, start := range kept {
			if start > t-20 {
				count++
			}
		}
		if count >= 8 {
			return false
		}
		kept = append(kept, t)
	}
	w.starts, w.last, w.end, w.initialized = kept, at, at+duration, true
	return true
}
