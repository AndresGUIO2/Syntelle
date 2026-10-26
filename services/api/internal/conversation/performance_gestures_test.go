package conversation

import (
	"encoding/binary"
	"math"
	"syntelle/render-contract"
	"testing"
)

func accentWAV() []byte {
	const rate = 8000
	wav := make([]byte, 44+4*rate*2)
	copy(wav, "RIFF")
	binary.LittleEndian.PutUint32(wav[4:], uint32(len(wav)-8))
	copy(wav[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(wav[16:], 16)
	binary.LittleEndian.PutUint16(wav[20:], 1)
	binary.LittleEndian.PutUint16(wav[22:], 1)
	binary.LittleEndian.PutUint32(wav[24:], rate)
	binary.LittleEndian.PutUint32(wav[28:], rate*2)
	binary.LittleEndian.PutUint16(wav[32:], 2)
	binary.LittleEndian.PutUint16(wav[34:], 16)
	copy(wav[36:], "data")
	binary.LittleEndian.PutUint32(wav[40:], 4*rate*2)
	for i := 0; i < 4*rate; i++ {
		at := float64(i) / rate
		envelope := math.Max(0, 1-math.Abs(at-2)/.16)
		value := int16(25000 * envelope * math.Sin(at*2*math.Pi*200))
		binary.LittleEndian.PutUint16(wav[44+i*2:], uint16(value))
	}
	return wav
}

func TestV3ExpressivePeaksFollowTheAcousticAccent(t *testing.T) {
	for _, kind := range []string{"nod", "shake", "emphasis", "brows", "tilt"} {
		t.Run(kind, func(t *testing.T) {
			amount := .8
			one := 1
			v := ValidatePerformance(PerformanceIntent{Version: "performance-intent/3", Tone: "neutral", Text: "aa mark aa", Cues: []PerformanceCue{{Kind: kind, Anchor: "mark", Amount: &amount, Repetitions: &one}}}, "aa mark aa")
			got := ResolvePerformance(v, accentWAV(), 4)
			if len(got.Resolved) != 1 {
				t.Fatal("spoken direction lost", got)
			}
			cue := got.Resolved[0]
			peak := cue.Time + rendercontract.NaturalGestureLead(kind, cue.Duration)
			if math.Abs(peak-2) > .05 {
				t.Fatal("gesture onset was aligned instead of its peak", cue, peak)
			}
			if cue.Time < 0 || cue.Time+cue.Duration > 4 || got.Timing != "audio-energy/2-approximate-gesture-peaks" {
				t.Fatal("invalid evidence/window", got)
			}
		})
	}
}

func TestV3UnplayableSequenceKeepsSpokenTextAndV2Timing(t *testing.T) {
	amount := 1.
	eight := 8
	intent := PerformanceIntent{Version: "performance-intent/3", Tone: "neutral", Text: "aa mark aa", Cues: []PerformanceCue{{Kind: "shake", Anchor: "mark", Amount: &amount, Repetitions: &eight}}}
	got := ResolvePerformance(ValidatePerformance(intent, intent.Text), accentWAV(), 4)
	if got.Intent.Text != intent.Text || len(got.Resolved) != 0 {
		t.Fatal("oversized sequence altered speech or escaped its window", got)
	}
	intent.Version = "performance-intent/2"
	intent.Cues[0].Repetitions = new(int(1))
	old := ResolvePerformance(ValidatePerformance(intent, intent.Text), accentWAV(), 4)
	if len(old.Resolved) != 1 || old.Resolved[0].Duration != .92 || old.Timing != "audio-energy/1-approximate-text-spans" {
		t.Fatal("legacy direction changed", old)
	}
}
