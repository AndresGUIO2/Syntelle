package nativeanimation

import (
	"math"
	"strings"
	"syntelle/render-contract"
	"unicode/utf16"
)

// AdmitPerformance reproduces the current private player's bounded performance
// admission. Laughter requires the prepared companion; voice/cooldown policy is
// applied before media delivery. The caller supplies a monotonic elapsed clock.
func (e *Engine) AdmitPerformance(value *Performance, text string, nowMS float64) *Performance {
	if value == nil || value.Intent == nil || (value.Intent.Version != "performance-intent/1" && !rendercontract.DirectedPerformance(value.Intent.Version)) || value.Intent.Text != text || value.Intent.Cues == nil || value.Resolved == nil {
		return nil
	}
	smiled := false
	heads := 0
	lastHead := math.Inf(-1)
	lastEnd := math.Inf(-1)
	cues := make([]PerformanceCue, 0, min(16, len(value.Resolved)))
	for _, cue := range value.Resolved[:min(16, len(value.Resolved))] {
		if rendercontract.DirectedPerformance(value.Intent.Version) && (cue.Amount == nil || math.IsNaN(*cue.Amount) || math.IsInf(*cue.Amount, 0) || *cue.Amount < 0 || *cue.Amount > 1) {
			continue
		}
		repeats := rendercontract.GestureRepetitions(cue.Kind, cue.Repetitions)
		if repeats == 0 {
			continue
		}
		maximumDuration := 2.5
		if cue.Kind == "nod" && cue.Repetitions != nil {
			maximumDuration = rendercontract.NodCycleSeconds*8 + .001
		}
		if cue.Kind == "shake" && cue.Repetitions != nil {
			maximumDuration = rendercontract.ShakeCycleSeconds*8 + .001
		}
		if value.Intent.Version == "performance-intent/3" && cue.Repetitions != nil {
			if cue.Kind == "nod" {
				maximumDuration = rendercontract.NaturalNodCycleSeconds*8 + .001
			}
			if cue.Kind == "shake" {
				maximumDuration = rendercontract.NaturalShakeCycleSeconds*8 + .001
			}
		}
		if math.IsNaN(cue.Time) || math.IsInf(cue.Time, 0) || cue.Time < 0 || math.IsNaN(cue.Duration) || math.IsInf(cue.Duration, 0) || cue.Duration <= 0 || cue.Duration > maximumDuration {
			continue
		}
		anchored := false
		if rendercontract.DirectedPerformance(value.Intent.Version) && cue.Kind == "emphasis" && cue.Source == "acoustic" {
			anchored = true
		}
		for _, anchor := range value.Intent.Cues {
			if anchor.Kind == cue.Kind && rendercontract.GestureRepetitions(anchor.Kind, anchor.Repetitions) == repeats && anchor.Anchor != "" && strings.Count(text, anchor.Anchor) == 1 {
				anchored = true
				break
			}
		}
		if !anchored {
			continue
		}
		if rendercontract.DirectedPerformance(value.Intent.Version) && (cue.Kind == "brows" || cue.Kind == "smile") {
			if cue.Kind == "smile" && !e.config.SmileAvailable {
				continue
			}
			cues = append(cues, cue)
			continue
		}
		if cue.Kind == "smile" {
			if smiled || nowMS+cue.Time*1000-e.lastSmile < 30000 || value.Intent.Tone == "serious" {
				continue
			}
			smiled = true
			cues = append(cues, cue)
			continue
		}
		if cue.Kind == "laugh" {
			if e.config.LaughAvailable && rendercontract.DirectedPerformance(value.Intent.Version) {
				cues = append(cues, cue)
			}
			continue
		}
		if !isHeadCue(cue.Kind) || heads+repeats > 8 || cue.Time-lastHead < 2.2 || cue.Time < lastEnd {
			continue
		}
		lastHead = cue.Time
		heads += repeats
		lastEnd = cue.Time + cue.Duration
		cues = append(cues, cue)
	}
	if smiled {
		for _, cue := range cues {
			if cue.Kind == "smile" {
				e.lastSmile = nowMS + cue.Time*1000
				break
			}
		}
	}
	out := *value
	out.Resolved = cues
	return &out
}

func isHeadCue(kind string) bool {
	return kind == "nod" || kind == "double_nod" || kind == "shake" || kind == "emphasis" || kind == "tilt"
}

// SpeechSeed matches [...('syntelle'+text)].reduce(...charCodeAt(0),17),
// including the first UTF-16 code unit of supplementary code points.
func SpeechSeed(text string) uint32 {
	seed := uint32(17)
	for _, r := range "syntelle" + text {
		code := r
		if r > 0xffff {
			high, _ := utf16.EncodeRune(r)
			code = high
		}
		seed = seed*31 + uint32(code)
	}
	return seed
}
