package nativerender

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"testing"
)

func TestLaughterPresentationReturnsDirectlyToLiveBaseline(t *testing.T) {
	root := os.Getenv("SYNTELLE_LAUGHTER_ASSETS")
	if root == "" {
		t.Skip("private complete laughter fixture")
	}
	a, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	r, err := newRenderer(a, "cpu")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if a.Expression.ReferenceLaughter.Presentation == nil {
		t.Fatal("new companion absent")
	}
	state := FrameState{From: "Rest", To: "A", Background: "dark-lounge"}
	state.Emotion[2] = .5
	base := image.NewRGBA(image.Rect(0, 0, 720, 960))
	again := image.NewRGBA(base.Rect)
	if err = r.Render(t.Context(), state, base); err != nil {
		t.Fatal(err)
	}
	for _, q := range []float64{.1, .5, 1, .5, .1} {
		state.Laugh = q
		state.JawOpen = .4
		if err = r.Render(t.Context(), state, again); err != nil {
			t.Fatal(err)
		}
	}
	state.Laugh = 0
	state.JawOpen = 0
	if err = r.Render(t.Context(), state, again); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(base.Pix, again.Pix) {
		t.Fatal("laughter did not restore the selected face exactly")
	}
	p := a.Expression.ReferenceLaughter.Presentation
	dx, dy := r.laughterDelta(a.Motion.Mandible.Chin[0], a.Motion.Mandible.Chin[1])
	_ = dx
	if dy > .1 || dy < -.1 {
		t.Fatal("reference reintroduced gross chin travel", dy, p.Validation)
	}
}

func TestLaughterCompositionHasNoFrameFeedback(t *testing.T) {
	root := os.Getenv("SYNTELLE_LAUGHTER_ASSETS")
	if root == "" {
		t.Skip("private cached companion")
	}
	a, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	r, err := newRenderer(a, "cpu")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	state := FrameState{From: "Rest", To: "A", Laugh: .5, LaughSpeech: true, Background: "dark-lounge"}
	state.Emotion[2] = .5
	first, again := image.NewRGBA(image.Rect(0, 0, 720, 960)), image.NewRGBA(image.Rect(0, 0, 720, 960))
	if err = r.Render(t.Context(), state, first); err != nil {
		t.Fatal(err)
	}
	if err = r.Render(t.Context(), state, again); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Pix, again.Pix) {
		t.Fatal("laughter composition reused the previous displayed face as its base")
	}
}

func TestLaughterEndpointContinuity(t *testing.T) {
	root := os.Getenv("SYNTELLE_LAUGHTER_ASSETS")
	if root == "" {
		t.Skip("private cached companion")
	}
	a, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	r, err := newRenderer(a, "cpu")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, resting := range []bool{false, true} {
		for _, smile := range []float64{0, .5, 1} {
			state := FrameState{From: "Rest", To: "A", Smile: smile, Background: "dark-lounge"}
			if !resting {
				state.Emotion[2] = .5
			}
			state.RestingSmile = resting
			var previous []byte
			var deltas []float64
			for _, q := range []float64{3. / 120, 2. / 120, 1. / 120, 0} {
				state.Laugh = q
				frame := image.NewRGBA(image.Rect(0, 0, 720, 960))
				if err = r.Render(t.Context(), state, frame); err != nil {
					t.Fatal(err)
				}
				if previous != nil {
					sum := 0.
					for y := 200; y < 650; y++ {
						for x := 190; x < 540; x++ {
							for c := 0; c < 3; c++ {
								i := y*frame.Stride + x*4 + c
								sum += math.Abs(float64(frame.Pix[i]) - float64(previous[i]))
							}
						}
					}
					deltas = append(deltas, sum/(450*350*3))
				}
				if path := os.Getenv("SYNTELLE_INSPECTION"); path != "" {
					f, _ := os.Create(fmt.Sprintf("%s/resting-%t-smile-%g-laugh-%g.png", path, resting, smile, q))
					png.Encode(f, frame)
					f.Close()
				}
				previous = append([]byte(nil), frame.Pix...)
			}
			t.Log("resting", resting, "smile", smile, "equal-step mean face deltas", deltas)
			if deltas[2] > math.Max(.3, deltas[1]*2) {
				data, _ := json.Marshal(deltas)
				t.Errorf("endpoint has an extra switch at smile %g: %s", smile, data)
			}
		}
	}

}
func TestLaughterKeepsUpperMaterialAttachedToHead(t *testing.T) {
	root := os.Getenv("SYNTELLE_LAUGHTER_ASSETS")
	if root == "" {
		t.Skip("cached companion")
	}
	a, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	r, err := newRenderer(a, "cpu")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	state := FrameState{From: "Rest", To: "A", Smile: .5, Background: "dark-lounge"}
	state.Emotion[2] = .5
	image := image.NewRGBA(image.Rect(0, 0, 720, 960))
	for _, q := range []float64{.2, .5, 1} {
		state.Laugh = q
		state.JawOpen = .6
		if err = r.Render(t.Context(), state, image); err != nil {
			t.Fatal(err)
		}
		got := r.referenceState(state)
		if got.upper != r.oral.shadeState.upper {
			t.Fatal("upper enamel moved with facial intensity", q, got.upper, r.oral.shadeState.upper)
		}
	}
}

func TestLaughterSoftTissueCannotBlendBaselineTeeth(t *testing.T) {
	root := os.Getenv("SYNTELLE_LAUGHTER_ASSETS")
	if root == "" {
		t.Skip("cached companion")
	}
	a, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	ordinary, err := newRenderer(a, "cpu")
	if err != nil {
		t.Fatal(err)
	}
	defer ordinary.Close()
	sentinel, err := newRenderer(a, "cpu")
	if err != nil {
		t.Fatal(err)
	}
	defer sentinel.Close()
	copyAssets := *a.Oral
	change := func(original *Texture) *Texture {
		copy := *original
		copy.Pixels = append([]byte(nil), original.Pixels...)
		for i := 0; i < len(copy.Pixels); i += 4 {
			copy.Pixels[i] = copy.Pixels[i+3]
			copy.Pixels[i+1] = 0
			copy.Pixels[i+2] = copy.Pixels[i+3]
		}
		return &copy
	}
	copyAssets.upper = change(a.Oral.upper)
	copyAssets.lower = change(a.Oral.lower)
	sentinel.oral.assets = &copyAssets
	state := FrameState{From: "Rest", To: "A", Smile: .85, Laugh: .4, JawOpen: .6, Background: "dark-lounge"}
	state.Emotion[2] = 1
	one, two := image.NewRGBA(image.Rect(0, 0, 720, 960)), image.NewRGBA(image.Rect(0, 0, 720, 960))
	if err = ordinary.Render(t.Context(), state, one); err != nil {
		t.Fatal(err)
	}
	if err = sentinel.Render(t.Context(), state, two); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(one.Pix, two.Pix) {
		t.Fatal("baseline enamel leaked into the soft-tissue blend; reference rows must own all active teeth")
	}
}
