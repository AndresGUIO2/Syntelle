package nativeanimation

import (
	"math"
	"syntelle/render-contract"
)

// EmotionSnapshot is immutable once handed to the video owner. Both clocks use
// the session's pause-aware audio time, never phrase-local or wall time.
type EmotionSnapshot struct {
	From, To             rendercontract.EmotionWeights
	Start, Duration      float64
	Bridge               bool
	EnergyFrom, EnergyTo float64
	Effective            rendercontract.EmotionTarget
	Phase                string
}

func (s EmotionSnapshot) Sample(ms float64) (rendercontract.EmotionWeights, float64) {
	if s.Duration <= 0 {
		return s.To, s.EnergyTo
	}
	t := ease((ms - s.Start) / math.Max(1, s.Duration))
	from, to := s.From, s.To
	if s.Bridge {
		if ms < s.Start+500 {
			t = ease((ms - s.Start) / 500)
			to = rendercontract.EmotionWeights{}
		} else {
			t = ease((ms - s.Start - 500) / 500)
			from = rendercontract.EmotionWeights{}
		}
	}
	var out rendercontract.EmotionWeights
	for i := range out {
		out[i] = blend(from[i], to[i], t)
	}
	return out, blend(s.EnergyFrom, s.EnergyTo, ease((ms-s.Start)/math.Max(1, s.Duration)))
}
func (s EmotionSnapshot) Negative(ms float64) bool {
	w, _ := s.Sample(ms)
	for _, i := range []int{5, 6, 7, 9} {
		if w[i] > .001 {
			return true
		}
	}
	return false
}

type emotionPolicyKey struct {
	Base               rendercontract.EmotionBaseline
	Regulation         rendercontract.EmotionRegulation
	Automatic, Reduced bool
}

// EmotionController has one session owner. Value copies stage complete audio
// blocks transactionally; failed sends cannot spend budgets or confirmations.
type EmotionController struct {
	Motion             EmotionSnapshot
	Revision, Sequence uint64
	key                emotionPolicyKey
	initialized        bool
	lastChange         float64
	changes            [2]float64
	changeCount        int
	confirmation       string
	confirmationAt     float64
	releaseAt          float64
	releasing          bool
	SuppressedTurn     string
	lastTurn           string
}

func baseTarget(b rendercontract.EmotionBaseline) rendercontract.EmotionTarget {
	b = b.Normalized()
	return rendercontract.EmotionTarget{State: b.State, Intensity: b.Intensity, Energy: .5}
}
func (c *EmotionController) move(ms float64, target rendercontract.EmotionTarget, duration float64, phase string, bridge bool) {
	w, energy := c.Motion.Sample(ms)
	c.Motion = EmotionSnapshot{From: w, To: emotionWeights(target), Start: ms, Duration: duration, Bridge: bridge, EnergyFrom: energy, EnergyTo: target.Energy, Effective: target, Phase: phase}
}
func (c *EmotionController) Configure(ms float64, settings rendercontract.Settings, turn string) bool {
	k := emotionPolicyKey{settings.EmotionBaseline.Normalized(), settings.EmotionRegulation.Normalized(), settings.AutomaticExpressions, settings.ReducedMotion}
	if c.initialized && c.key == k {
		return false
	}
	if c.initialized && (c.key.Base != k.Base || c.key.Regulation != k.Regulation) {
		c.SuppressedTurn = turn
	}
	c.key = k
	c.Revision++
	c.confirmation = ""
	c.releasing = false
	duration := 900.
	if !c.initialized || k.Reduced {
		duration = 0
	}
	if !c.initialized {
		c.Motion.EnergyFrom = .5
		c.Motion.EnergyTo = .5
	}
	c.move(ms, baseTarget(k.Base), duration, "baseline", false)
	c.initialized = true
	return true
}
func (c *EmotionController) Controlled(turn string) bool {
	return c.key.Regulation.Enabled || turn != "" && c.SuppressedTurn == turn
}
func (c *EmotionController) End(ms float64, interrupted bool) {
	if interrupted {
		c.confirmation = ""
	}
	if c.Motion.Phase != "baseline" && !c.releasing {
		c.releaseAt = ms + 2000
		c.Motion.Phase = "holding"
		c.releasing = true
	}
}
func (c *EmotionController) Advance(ms float64, busy bool) bool {
	changed := false

	if c.releasing && ms >= c.releaseAt && c.Motion.Phase != "recovery" {
		c.move(c.releaseAt, baseTarget(c.key.Base), c.key.Regulation.Profile().RecoveryMS, "recovery", false)
		changed = true
	}
	if c.Motion.Phase == "recovery" && ms >= c.Motion.Start+c.Motion.Duration {
		c.move(ms, baseTarget(c.key.Base), 0, "baseline", false)
		c.releasing = false
		changed = true
	}
	if c.Motion.Phase == "bridge" && ms >= c.Motion.Start+c.Motion.Duration {
		c.Motion.Phase = "reaction"
		changed = true
	}
	return changed
}
func emotionFamily(state string) int {
	switch state {
	case "joyful", "playful", "affectionate":
		return 1
	case "sad", "angry", "fearful", "disgusted":
		return -1
	}
	return 0
}
func (c *EmotionController) Evidence(proposed *rendercontract.EmotionTarget, reason, turn string, phrase int, generation uint64) rendercontract.EmotionDecision {
	c.Sequence++
	return rendercontract.EmotionDecision{Version: rendercontract.EmotionPolicyVersion, PolicyRevision: c.Revision, Sequence: c.Sequence, TurnID: turn, PhraseIndex: phrase, Generation: generation, Baseline: c.key.Base, Regulation: c.key.Regulation, Proposed: proposed, Effective: c.Motion.Effective, Phase: c.Motion.Phase, Reason: reason}
}
func (c *EmotionController) Propose(ms float64, proposed *rendercontract.EmotionTarget, turn string, phrase int, generation uint64) rendercontract.EmotionDecision {
	reason := "invalid"
	defer func() { c.lastTurn = turn }()
	if proposed == nil || !proposed.Valid() {
		c.confirmation = ""
		return c.Evidence(proposed, reason, turn, phrase, generation)
	}
	if c.SuppressedTurn == turn || !c.key.Automatic || c.key.Reduced {
		c.confirmation = ""
		return c.Evidence(proposed, "suppressed", turn, phrase, generation)
	}
	target := *proposed
	target.Intensity = math.Min(target.Intensity, rendercontract.EmotionCeiling(target.State)) * c.key.Regulation.Profile().Multiplier
	target.Energy = math.Max(.35, math.Min(.65, target.Energy))
	if target.State != "neutral" && target.Intensity < .10 {
		c.confirmation = ""
		return c.Evidence(proposed, "below_threshold", turn, phrase, generation)
	}
	current := c.Motion.Effective
	if c.Motion.Phase == "recovery" {
		w, _ := c.Motion.Sample(ms)
		dominant := 0
		for i := 1; i < len(w); i++ {
			if w[i] > w[dominant] {
				dominant = i
			}
		}
		if w[dominant] > .001 {
			current.State = rendercontract.EmotionStates[dominant]
			current.Intensity = w[dominant]
		}
	}
	if target.State == current.State {
		c.confirmation = ""
		if math.Abs(target.Intensity-current.Intensity) < .08 && c.Motion.Phase != "recovery" {
			reason := "hysteresis"
			if c.releasing {
				c.releasing = false
				c.Motion.Phase = "reaction"
				reason = "sustained"
			}
			return c.Evidence(proposed, reason, turn, phrase, generation)
		}
		sample, _ := c.Motion.Sample(ms)
		duration := math.Max(500, 1.875*math.Abs(target.Intensity-sample[rendercontract.EmotionIndex(target.State)])/.15*1000)
		c.move(ms, target, duration, "reaction", false)
		c.releasing = false
		return c.Evidence(proposed, "intensity_limited", turn, phrase, generation)
	}
	bridge := emotionFamily(current.State)*emotionFamily(target.State) == -1
	confirmed := false
	if bridge {
		confirmed = c.confirmation == target.State && ms-c.confirmationAt <= 10000
		if c.confirmation != target.State || ms-c.confirmationAt > 10000 {
			c.confirmation = target.State
			c.confirmationAt = ms
		}
	} else {
		c.confirmation = ""
	}
	if c.changeCount > 0 && ms-c.lastChange < c.key.Regulation.Profile().DwellMS {
		return c.Evidence(proposed, "minimum_duration", turn, phrase, generation)
	}
	count := 0
	for i := 0; i < c.changeCount; i++ {
		if ms-c.changes[i] < 20000 {
			c.changes[count] = c.changes[i]
			count++
		}
	}
	c.changeCount = count
	if count == 2 {
		return c.Evidence(proposed, "change_budget", turn, phrase, generation)
	}
	if bridge && !confirmed {
		return c.Evidence(proposed, "confirmation_required", turn, phrase, generation)
	}
	c.confirmation = ""
	c.changes[c.changeCount] = ms
	c.changeCount++
	c.lastChange = ms
	c.releasing = false
	duration, phase := 500., "reaction"
	if bridge {
		duration, phase = 1000, "bridge"
	}
	c.move(ms, target, duration, phase, bridge)
	return c.Evidence(proposed, "accepted", turn, phrase, generation)
}

func (c *EmotionController) HasPlayedProposal(turn string) bool {
	return turn != "" && c.lastTurn == turn
}
