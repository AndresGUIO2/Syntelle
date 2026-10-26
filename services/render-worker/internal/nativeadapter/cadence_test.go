package nativeadapter

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"syntelle/render-contract"
	"syntelle/render-worker/internal/nativemedia"
)

func TestPreviewPoseAdvancesWithFrameClock(t *testing.T) {
	var evidence []map[string]any
	for _, kind := range []string{"nod", "shake"} {
		s, _ := fixtureScene(t)
		settings := rendercontract.Defaults()
		settings.Movement, settings.Blinking, settings.AutomaticExpressions = false, false, false
		input := nativemedia.FrameInput{Settings: settings, PreviewKind: kind, PreviewAmount: 1, PreviewRepetitions: 4, PreviewSequence: 1}
		previous := [3]float64{}
		unchanged := 0
		for frame := 0; frame < 60; frame++ {
			input.MediaTime = time.Duration(frame) * time.Second / 30
			state, err := s.sample(input)
			if err != nil {
				t.Fatal(err)
			}
			if frame > 0 && state.Head == previous {
				unchanged++
			} else {
				unchanged = 0
			}
			if unchanged >= 3 {
				t.Fatal("advancing frame clock produced a frozen gesture", kind, frame)
			}
			previous = state.Head
			evidence = append(evidence, map[string]any{"kind": kind, "media_time_ns": input.MediaTime, "head": state.Head})
		}
	}
	if path := os.Getenv("SYNTELLE_CADENCE_EVIDENCE"); path != "" {
		data, err := json.MarshalIndent(evidence, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
