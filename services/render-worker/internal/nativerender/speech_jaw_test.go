package nativerender

import (
	"bytes"
	"encoding/json"
	"image"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func speechJawAssets(t *testing.T) (*Assets, [2]float64, float64) {
	t.Helper()
	root := os.Getenv("SYNTELLE_SPEECH_JAW_ASSETS")
	if root == "" {
		t.Skip("explicit private speech-only jaw package")
	}
	a, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Packages struct {
			Motion struct {
				SpeechJaw struct {
					Chin [2]float64
					Neck float64 `json:"lower_neck_y"`
				} `json:"speech_jaw"`
			} `json:"motion"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	return a, manifest.Packages.Motion.SpeechJaw.Chin, manifest.Packages.Motion.SpeechJaw.Neck
}

func TestSpeechJawMovesOnlySourceBoundLowerFace(t *testing.T) {
	a, chin, neck := speechJawAssets(t)
	r, err := newRenderer(a, "cpu")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	state := FrameState{From: "Rest", To: "A", Amount: 1}
	r.prepareBody(state)
	base := append([]rasterVertex(nil), r.vertices...)
	if err := json.Unmarshal([]byte(`{"SpeechJaw":1}`), &state); err != nil {
		t.Fatal(err)
	}
	r.prepareBody(state)
	nearest, best := 0, math.Inf(1)
	maximum := 0.
	for i, v := range a.vertices {
		x, y := v.X*float64(a.Motion.Canvas.Width), v.Y*float64(a.Motion.Canvas.Height)
		d := math.Hypot(x-chin[0], y-chin[1])
		if d < best {
			nearest, best = i, d
		}
		change := math.Hypot(r.vertices[i].X-base[i].X, r.vertices[i].Y-base[i].Y)
		maximum = math.Max(maximum, change)
		crop := r.mouthPatch.Rect
		oral := speechJawProtectedPoint(a, x, y)
		upper := float64(crop.Y)
		if a.Motion.SpeechJaw.OralSupport != nil {
			upper = math.Inf(1)
			for _, p := range a.Motion.SpeechJaw.OralSupport.Polygon {
				upper = math.Min(upper, p[1])
			}
		}
		if (oral || y >= neck || y < upper || v.Head == 0) && change > 1e-8 {
			t.Fatalf("speech jaw moved protected source geometry at %g,%g by %g", x, y, change)
		}
	}
	travel := r.vertices[nearest].Y - base[nearest].Y
	if travel < .1 {
		t.Fatal("speech jaw has no visible chin articulation", travel, maximum)
	}
	if travel > float64(r.body.Height)/float64(a.Motion.Canvas.Height)*a.Motion.Anatomy.FaceHeight*.0075+1e-6 {
		t.Fatal("speech jaw exceeds subtle calibrated range", travel)
	}
	t.Logf("source-bound chin travel %.6f output px, maximum lower-face movement %.6f px", travel, maximum)
	beforeImage, afterImage := image.NewRGBA(image.Rect(0, 0, 720, 960)), image.NewRGBA(image.Rect(0, 0, 720, 960))
	beforeState := state
	if err := json.Unmarshal([]byte(`{"SpeechJaw":0}`), &beforeState); err != nil {
		t.Fatal(err)
	}
	if err := r.Render(t.Context(), beforeState, beforeImage); err != nil {
		t.Fatal(err)
	}
	if err := r.Render(t.Context(), state, afterImage); err != nil {
		t.Fatal(err)
	}
	crop := r.mouthPatch.Rect
	left := int(math.Ceil(r.x + float64(crop.X)*r.drawWidth/float64(a.Motion.Canvas.Width)))
	right := int(math.Floor(r.x + float64(crop.X+crop.Width)*r.drawWidth/float64(a.Motion.Canvas.Width)))
	top := int(math.Ceil(r.y + float64(crop.Y)*r.drawHeight/float64(a.Motion.Canvas.Height)))
	bottom := int(math.Floor(r.y + float64(crop.Y+crop.Height)*r.drawHeight/float64(a.Motion.Canvas.Height)))
	for y := top; y < bottom; y++ {
		for x := left; x < right; x++ {
			sourceX := (float64(x) + .5 - r.x) * float64(a.Motion.Canvas.Width) / r.drawWidth
			sourceY := (float64(y) + .5 - r.y) * float64(a.Motion.Canvas.Height) / r.drawHeight
			if !speechJawProtectedPoint(a, sourceX, sourceY) {
				continue
			}
			at := y*beforeImage.Stride + x*4
			if !bytes.Equal(beforeImage.Pix[at:at+4], afterImage.Pix[at:at+4]) {
				t.Fatalf("skin-only jaw changed original mouth/teeth pixels at %d,%d", x, y)
			}
		}
	}
}

func speechJawProtectedPoint(a *Assets, x, y float64) bool {
	rig := a.Motion.SpeechJaw
	if rig.OralSupport != nil {
		p := [2]float64{x, y}
		return speechJawSupportOverlaps([3][2]float64{p, p, p}, rig.OralSupport.Polygon, rig.OralSupport.SamplingGuard)
	}
	c := rig.ProtectedOralCrop
	return x >= float64(c.X) && x <= float64(c.X+c.Width) && y >= float64(c.Y) && y <= float64(c.Y+c.Height)
}

func TestSpeechJawCentralSkinBridgeIsContinuous(t *testing.T) {
	a, chin, _ := speechJawAssets(t)
	if a.Motion.SpeechJaw.Version != "speech-jaw/2" {
		t.Skip("version1 retains its historical rectangular exclusion")
	}
	r, err := newRenderer(a, "cpu")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	state := FrameState{From: "Rest", To: "A", Amount: 1}
	r.prepareBody(state)
	base := append([]rasterVertex(nil), r.vertices...)
	state.SpeechJaw = 1
	r.prepareBody(state)
	bottom := math.Inf(-1)
	for _, p := range a.Motion.SpeechJaw.OralSupport.Polygon {
		bottom = math.Max(bottom, p[1])
	}
	previousY, previousDelta := 0., 0.
	for i, fraction := range []float64{.5, .65, .8} {
		targetY := bottom + (chin[1]-bottom)*fraction
		nearest, best := 0, math.Inf(1)
		for j, v := range a.vertices {
			d := math.Hypot(v.X*float64(a.Motion.Canvas.Width)-chin[0], v.Y*float64(a.Motion.Canvas.Height)-targetY)
			if d < best {
				nearest, best = j, d
			}
		}
		v := a.vertices[nearest]
		y := v.Y * float64(a.Motion.Canvas.Height)
		delta := (r.vertices[nearest].Y - base[nearest].Y) * float64(a.Motion.Canvas.Height) / float64(r.body.Height)
		if a.Motion.SpeechJaw.Weights[nearest] <= .001 || delta <= 0 {
			t.Fatal("central skin remains a frozen island", fraction, y, delta)
		}
		if i > 0 {
			gradient := math.Abs(delta-previousDelta) / math.Max(1e-8, y-previousY)
			if gradient > .15 {
				t.Fatal("central bridge displacement changes abruptly", fraction, gradient)
			}
		}
		t.Logf("bridge fraction %.2f y%.2f weight%.4f displacement%.4f sourcepx", fraction, y, a.Motion.SpeechJaw.Weights[nearest], delta)
		previousY, previousDelta = y, delta
	}
}

func TestSpeechJawRejectsFrozenCentralSkinOrStaleSupport(t *testing.T) {
	for _, name := range []string{"frozen-bridge", "stale-lip-support"} {
		t.Run(name, func(t *testing.T) {
			a, chin, _ := speechJawAssets(t)
			if a.Motion.SpeechJaw.Version != "speech-jaw/2" {
				t.Skip("version2 source support fixture")
			}
			if name == "stale-lip-support" {
				a.Motion.SpeechJaw.OralSupport.Polygon[0][0] += 1
			} else {
				for i, v := range a.vertices {
					if math.Abs(v.X*float64(a.Motion.Canvas.Width)-chin[0]) < 20 && v.Y*float64(a.Motion.Canvas.Height) < chin[1] {
						a.Motion.SpeechJaw.Weights[i] = 0
					}
				}
			}
			if err := a.validateSpeechJaw(); err == nil {
				t.Fatal("invalid source support/skin continuity was accepted")
			}
		})
	}
}

func TestSpeechJawLegacyResourceRetainsLoadability(t *testing.T) {
	path := os.Getenv("SYNTELLE_SPEECH_JAW_LEGACY")
	if path == "" {
		t.Skip("explicit legacy source-bound jaw fixture")
	}
	a, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if a.Motion.SpeechJaw == nil || a.Motion.SpeechJaw.Version != "speech-jaw/1" {
		t.Fatal("wrong legacy fixture")
	}
}

func TestSpeechJawProtectsCalibratedLipPixelsAcrossPosesAndSmiles(t *testing.T) {
	a, _, _ := speechJawAssets(t)
	r, err := newRenderer(a, "cpu")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	before, after := image.NewRGBA(image.Rect(0, 0, 720, 960)), image.NewRGBA(image.Rect(0, 0, 720, 960))
	protectedOffsets := []int{}
	crop := r.mouthPatch.Rect
	left := int(math.Ceil(r.x + float64(crop.X)*r.drawWidth/float64(a.Motion.Canvas.Width)))
	right := int(math.Floor(r.x + float64(crop.X+crop.Width)*r.drawWidth/float64(a.Motion.Canvas.Width)))
	top := int(math.Ceil(r.y + float64(crop.Y)*r.drawHeight/float64(a.Motion.Canvas.Height)))
	bottom := int(math.Floor(r.y + float64(crop.Y+crop.Height)*r.drawHeight/float64(a.Motion.Canvas.Height)))
	for y := top; y < bottom; y++ {
		for x := left; x < right; x++ {
			sx := (float64(x) + .5 - r.x) * float64(a.Motion.Canvas.Width) / r.drawWidth
			sy := (float64(y) + .5 - r.y) * float64(a.Motion.Canvas.Height) / r.drawHeight
			if speechJawProtectedPoint(a, sx, sy) {
				protectedOffsets = append(protectedOffsets, y*before.Stride+x*4)
			}
		}
	}
	for _, pose := range []string{"A", "MBP", "FV", "EI", "U", "O"} {
		for _, smile := range []float64{0, .5, 1} {
			for _, emotion := range []float64{0, .5, 1} {
				state := FrameState{From: "Rest", To: pose, Amount: 1, Smile: smile}
				state.Emotion[2] = emotion
				if err := r.Render(t.Context(), state, before); err != nil {
					t.Fatal(err)
				}
				state.SpeechJaw = 1
				if err := r.Render(t.Context(), state, after); err != nil {
					t.Fatal(err)
				}
				for _, at := range protectedOffsets {
					if !bytes.Equal(before.Pix[at:at+4], after.Pix[at:at+4]) {
						t.Fatalf("jaw changed actual lip/enamel support at pose%s smile%g emotion%g offset%d", pose, smile, emotion, at)
					}
				}
			}
		}
	}
}

func TestSpeechJawZeroPreservesOriginalFaceAndLaughter(t *testing.T) {
	a, _, _ := speechJawAssets(t)
	oldRoot := os.Getenv("SYNTELLE_SPEECH_JAW_PREVIOUS")
	if oldRoot == "" {
		t.Skip("explicit original phase107 package")
	}
	old, err := Load(oldRoot)
	if err != nil {
		t.Fatal(err)
	}
	r, err := newRenderer(a, "cpu")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	before, err := newRenderer(old, "cpu")
	if err != nil {
		t.Fatal(err)
	}
	defer before.Close()
	one, two := image.NewRGBA(image.Rect(0, 0, 720, 960)), image.NewRGBA(image.Rect(0, 0, 720, 960))
	for _, row := range []struct {
		name  string
		state FrameState
	}{
		{"rest", FrameState{From: "Rest", To: "A"}},
		{"open-A", FrameState{From: "Rest", To: "A", Amount: 1}},
		{"MBP", FrameState{From: "Rest", To: "MBP", Amount: 1}},
		{"FV", FrameState{From: "Rest", To: "FV", Amount: 1}},
		{"manual-smile", FrameState{From: "Rest", To: "A", Smile: 1, RestingSmile: true}},
		{"laugh-half", FrameState{From: "Rest", To: "A", Laugh: .5, Amount: .3}},
		{"laugh-full", FrameState{From: "Rest", To: "A", Laugh: 1, Amount: .3}},
	} {
		t.Run(row.name, func(t *testing.T) {
			if err := r.Render(t.Context(), row.state, one); err != nil {
				t.Fatal(err)
			}
			if err := before.Render(t.Context(), row.state, two); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(one.Pix, two.Pix) {
				t.Fatal("zero speech jaw changed the preserved phase107 face/laughter")
			}
		})
	}
}
