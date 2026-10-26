//go:build integration

package nativeadapter

import (
	"encoding/json"
	"os"
	"syntelle/render-contract"
	"syntelle/render-worker/internal/nativeanimation"
	"syntelle/render-worker/internal/nativemedia"
	"syntelle/render-worker/internal/nativerender"
	"testing"
	"time"
)

func TestRecordNaturalGestureFrames(t *testing.T) {
	path, out := os.Getenv("SYNTELLE_GESTURE_PACKAGE"), os.Getenv("SYNTELLE_GESTURE_STATES")
	if path == "" || out == "" {
		t.Skip("explicit cached gesture recording only")
	}
	assets, err := nativerender.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	config, err := nativeanimation.LoadBundle(assets.MotionJSON, assets.GazeJSON, assets.ExpressionJSON, assets.OralJSON)
	if err != nil {
		t.Fatal(err)
	}
	settings := rendercontract.Defaults()
	settings.Movement, settings.Blinking, settings.AutomaticExpressions = false, false, false
	type recorded struct {
		Name  string
		State nativerender.FrameState
	}
	states := []recorded{{Name: "neutral", State: nativerender.FrameState{From: "Rest", To: "A"}}}
	for _, kind := range []string{"nod", "shake", "tilt", "emphasis"} {
		for i, at := range []float64{.12, .28, .50, .76} {
			s := &scene{config: config, engine: nativeanimation.NewEngine(config)}
			input := nativemedia.FrameInput{Settings: settings, PreviewKind: kind, PreviewSequence: 1, PreviewAmount: .85, PreviewRepetitions: 1}
			if _, err = s.sample(input); err != nil {
				t.Fatal(err)
			}
			input.MediaTime = time.Duration(at * float64(time.Second))
			state, err := s.sample(input)
			if err != nil {
				t.Fatal(err)
			}
			pose := []string{"Rest", "MBP", "FV", "A"}[i]
			state.From, state.To = pose, pose
			state.Brows = .6
			states = append(states, recorded{Name: kind + "-" + pose, State: state})
		}
	}
	states = append(states, recorded{Name: "brows-FV", State: nativerender.FrameState{From: "FV", To: "FV", Brows: 1}}, recorded{Name: "brows-A", State: nativerender.FrameState{From: "Rest", To: "A", Amount: .75, Brows: 1}})
	raw, err := json.MarshalIndent(states, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(out, raw, 0600); err != nil {
		t.Fatal(err)
	}
}
