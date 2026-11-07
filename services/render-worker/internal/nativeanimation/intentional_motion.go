package nativeanimation

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

type intentionalAdmissionState struct {
	reply, phrase string
	seen          map[string]bool
	current       *Performance
}

// AdmitPerformanceUpdate admits only newly discovered future anchors. It never
// changes an already approved phrase direction or spends a cycle twice.
func (e *Engine) AdmitPerformanceUpdate(value *Performance, text string, nowMS, position float64) *Performance {
	if value == nil && e.intentionalState.current != nil && e.intentionalState.current.Intent != nil && e.intentionalState.current.Intent.Text == text {
		return e.intentionalState.current
	}
	if e.config.PresenceVersion != IntentionalPresenceVersion || value == nil || !value.Motion.Valid() {
		return e.admitPerformanceLegacy(value, text, nowMS)
	}
	return e.admitIntentional(value, text, nowMS, position)
}
func (e *Engine) admitIntentional(value *Performance, text string, nowMS, position float64) *Performance {
	if value == nil || value.Intent == nil || value.Intent.Text != text {
		return nil
	}
	state := &e.intentionalState
	reply := value.ReplyID
	if value.Motion.Valid() {
		reply = value.Motion.ReplyID
	}
	if state.seen == nil || state.reply != reply {
		*state = intentionalAdmissionState{reply: reply, seen: map[string]bool{}}
	}
	offset := 0.
	if value.Motion.Valid() {
		offset = value.Motion.Offset
	}
	phrase := fmt.Sprintf("%s|%.6f", text, offset)
	previous := state.current
	if state.phrase != phrase {
		previous = nil
		state.phrase = phrase
	}
	proposal := *value
	if previous != nil {
		proposal.Intent = previous.Intent
		proposal.Motion = previous.Motion
		proposal.ReplyID = previous.ReplyID
	}
	fresh := make([]PerformanceCue, 0, len(value.Resolved))
	for index, cue := range value.Resolved {
		key := cue.ID
		if key == "" {
			key = fmt.Sprintf("%s/legacy-%s-%d", phrase, cue.Kind, index)
		}
		if state.seen[key] {
			continue
		}
		state.seen[key] = true
		if isHeadCue(cue.Kind) {
			if value.MotionDirection != SemanticMotionDirection || cue.Source != "semantic" || !validMotionCue(cue) {
				continue
			}
			if position >= 0 && cue.Time < position+.02 {
				proposal.Omitted = append(proposal.Omitted, "late_motion_cue")
				continue
			}
		} else if position >= 0 && cue.Time < position {
			continue
		}
		fresh = append(fresh, cue)
	}
	proposal.Resolved = fresh
	admitted := e.admitPerformanceLegacy(&proposal, text, nowMS)
	if admitted == nil {
		return previous
	}
	if previous != nil {
		admitted.Resolved = append(append([]PerformanceCue(nil), previous.Resolved...), admitted.Resolved...)
	}
	sort.SliceStable(admitted.Resolved, func(i, j int) bool { return admitted.Resolved[i].Time < admitted.Resolved[j].Time })
	state.current = admitted
	return admitted
}
func validMotionCue(c PerformanceCue) bool {
	for _, v := range []float64{c.Time, c.Duration, c.Peak, c.AnchorStart, c.AnchorEnd, c.Confidence} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return c.ID != "" && len(c.ID) <= 128 && !strings.ContainsAny(c.ID, "\r\n") && c.Confidence >= .7 && c.Confidence <= 1 &&
		(c.Timing == "native-word-boundaries/1" || c.Timing == "approximate-acoustic-span/1" || c.Timing == "audible-phrase-boundary/1") && c.AnchorStart >= 0 && c.AnchorEnd >= c.AnchorStart && c.Peak >= c.AnchorStart && c.Peak <= c.AnchorEnd && c.Peak >= c.Time && c.Peak < c.Time+c.Duration
}

func intentionalGestureCue(plan *Plan, g Gesture) (string, *PerformanceCue) {
	if plan == nil || plan.Performance == nil {
		return "", nil
	}
	for i := range plan.Performance.Resolved {
		cue := &plan.Performance.Resolved[i]
		if cue.Kind != g.Kind || !validMotionCue(*cue) || cue.Source != "semantic" {
			continue
		}
		if g.Time+1e-6 >= cue.Time && g.Time < cue.Time+cue.Duration-1e-6 {
			return fmt.Sprintf("%s/%.6f", cue.ID, g.Time-cue.Time), cue
		}
	}
	return "", nil
}
func (p *presenceDirector) intentionalGesture(g Gesture, c Config, amount float64, purpose string) Gesture {
	// Geometry/headroom and the previous meaningful action choose the direction;
	// no random stream decides what the speaker emphasizes.
	sign := 1.
	if p.base.Yaw > .15 {
		sign = -1
	} else if p.base.Yaw < -.15 {
		sign = 1
	} else if p.lastSemanticSign != 0 {
		sign = -p.lastSemanticSign
	}
	p.lastSemanticSign = sign
	if g.Kind == "emphasis" {
		g.Yaw = sign * math.Min(6, c.Bounds.Yaw*.8) * amount
		g.Pitch = -math.Min(2, c.Bounds.Pitch*.6) * amount
		g.Roll = sign * math.Min(.7, c.Bounds.Roll*.65) * amount
	}
	if g.Kind == "tilt" {
		g.Yaw = sign * math.Min(3, c.Bounds.Yaw*.5) * amount
		g.Pitch = -math.Min(.5, c.Bounds.Pitch*.2) * amount
		g.Roll = sign * math.Min(.7, c.Bounds.Roll*.65) * amount
	}
	switch purpose {
	case "ask":
		g.Pitch *= -.7 // lift toward the viewer at the question's anchor
	case "reassure":
		g.Yaw *= .7
		g.Roll *= .55
	}
	if g.Kind == "shake" {
		g.Yaw = sign * math.Abs(g.Yaw)
	}
	return g
}
func intentionalGestureAt(g Gesture, phase float64) Head {
	if g.Kind != "emphasis" && g.Kind != "tilt" {
		return naturalGestureAt(g, phase)
	}
	if phase <= 0 || phase >= 1 {
		return Head{}
	}
	at := phase * g.Duration
	stroke := math.Max(.001, g.Duration-.32)
	w := ease(at / stroke)
	if at > stroke {
		w = 1 - ease((at-stroke)/.32)
	}
	return Head{Yaw: g.Yaw * w, Pitch: g.Pitch * w, Roll: g.Roll * w}
}
