package nativeanimation

import "syntelle/render-contract"

// ReservedEmotion is an immutable proposal evaluated against played state.
// Only ActivateReserved may commit its counters and confirmation deltas.
type ReservedEmotion struct {
	next     EmotionController
	before   EmotionController
	decision rendercontract.EmotionDecision
	settings rendercontract.Settings
	at       float64
}

func (c EmotionController) ReserveEmotion(ms float64, settings rendercontract.Settings, request rendercontract.DeliveryRequest, generation uint64, manual ...bool) (ReservedEmotion, rendercontract.EmotionDecision) {
	before := c
	c.Configure(ms, settings, request.TurnID)
	if len(manual) > 0 && manual[0] {
		c.SuppressedTurn = request.TurnID
	}
	var decision rendercontract.EmotionDecision
	if !settings.EmotionRegulation.Normalized().Enabled && !c.Controlled(request.TurnID) && settings.AutomaticExpressions && !settings.ReducedMotion && request.Emotion != nil && request.Emotion.Valid() {
		c.move(ms, *request.Emotion, 500, "reaction", false)
		c.lastTurn = request.TurnID
		decision = c.Evidence(request.Emotion, "accepted", request.TurnID, request.PhraseIndex, generation)
	} else {
		decision = c.Propose(ms, request.Emotion, request.TurnID, request.PhraseIndex, generation)
	}
	return ReservedEmotion{next: c, before: before, decision: decision, settings: settings, at: ms}, decision
}

func (c *EmotionController) ActivateReserved(ms float64, reservation ReservedEmotion) rendercontract.EmotionDecision {
	next := reservation.next
	// Counters belong to actual activation time. Existing historical timestamps
	// retain their values; reserve/evaluate never writes the live controller.
	if next.changeCount > reservation.before.changeCount || next.lastChange != reservation.before.lastChange {
		next.lastChange = ms
		if next.changeCount > 0 {
			next.changes[next.changeCount-1] = ms
		}
	}
	if next.confirmation != "" && (next.confirmation != reservation.before.confirmation || next.confirmationAt != reservation.before.confirmationAt) {
		next.confirmationAt = ms
	}
	if c.initialized && c.Motion.Effective == reservation.decision.Effective && c.Motion.Phase != "recovery" && c.key == next.key {
		next.Motion = c.Motion
	} else {
		from, energy := c.Motion.Sample(ms)
		if !c.initialized {
			from, energy = next.Motion.From, next.Motion.EnergyFrom
		}
		next.Motion.From, next.Motion.EnergyFrom, next.Motion.Start = from, energy, ms
	}
	next.Sequence = c.Sequence
	*c = next
	d := reservation.decision
	return c.Evidence(d.Proposed, d.Reason, d.TurnID, d.PhraseIndex, d.Generation)
}
