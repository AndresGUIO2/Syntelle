package nativerender

import (
	"bytes"
	"context"
	"image"
	"math"
	"os"
	"testing"
)

func TestCachedMandibularGeometryAndCanonicalRest(t *testing.T) {
	root := os.Getenv("SYNTELLE_NATIVE_ASSETS")
	old := os.Getenv("SYNTELLE_PREVIOUS_ASSETS")
	if root == "" || old == "" {
		t.Skip("explicit private preparation fixture")
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
	previous, err := Load(old)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := newRenderer(previous, "cpu")
	if err != nil {
		t.Fatal(err)
	}
	defer legacy.Close()
	state := FrameState{From: "Rest", To: "A"}
	one, two := image.NewRGBA(image.Rect(0, 0, 720, 960)), image.NewRGBA(image.Rect(0, 0, 720, 960))
	if err = r.Render(context.Background(), state, one); err != nil {
		t.Fatal(err)
	}
	if err = legacy.Render(context.Background(), state, two); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(one.Pix, two.Pix) {
		t.Fatal("canonical rest changed")
	}
	rig := a.Motion.Mandible
	nearest := 0
	best := math.Inf(1)
	for i, v := range a.vertices {
		d := math.Hypot(v.X*float64(a.Motion.Canvas.Width)-rig.Chin[0], v.Y*float64(a.Motion.Canvas.Height)-rig.Chin[1])
		if d < best {
			best, nearest = d, i
		}
	}
	r.prepareBody(state)
	base := r.vertices[nearest]
	state.JawOpen = rig.PoseJaw["A"]
	r.prepareBody(state)
	chin := r.vertices[nearest]
	travel := (chin.Y - base.Y) * 720 / float64(r.body.Height)
	if travel < 3 {
		t.Fatal("jaw not visible independently of head", travel)
	}
	for _, head := range [][3]float64{{0, 0, 0}, {8, 4, 1.2}, {-8, -4, -1.2}, {8, -4, 1.2}, {-8, 4, -1.2}} {
		for _, jaw := range []float64{0, .5, 1} {
			state.Head = head
			state.JawOpen = jaw
			r.prepareBody(state)
			for i := 0; i < len(a.indices); i += 3 {
				u, v, w := r.vertices[a.indices[i]], r.vertices[a.indices[i+1]], r.vertices[a.indices[i+2]]
				if !patchFinite(edge(u, v, w.X, w.Y)) {
					t.Fatal("nonfinite joint geometry")
				}
			}
		}
	}
	t.Logf("open-A chin travel at720=%.3f px", travel)
}

func TestMandibleResidualContactAndCoverage(t *testing.T) {
	root := os.Getenv("SYNTELLE_NATIVE_ASSETS")
	if root == "" {
		t.Skip("private fixture required")
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
	for _, pose := range []string{"Rest", "MBP", "FV", "EI", "U", "O", "A"} {
		var original [20][2]float64
		for n, jaw := range []float64{0, .5, 1} {
			state := FrameState{From: pose, To: "A", JawOpen: jaw, LipOpening: .05, LipTension: .05}
			if pose == "A" {
				state.From = "Rest"
				state.Amount = 1
			}
			r.configureArticulation(state)
			_, err = r.oral.drawExpression(state.From, state.To, state.Amount, 0, func(mesh *oralMesh, ids [3]int, points [3][2]float64, shade *oralShadeState) {
				if shade.lower[1] < shade.upper[1]+a.Motion.Mandible.LowerStop-1e-5 {
					t.Fatal("opposing enamel overlap", pose)
				}
				if pose == "Rest" || pose == "MBP" || pose == "FV" {
					if n == 0 {
						original = shade.inner
					} else if original != shade.inner {
						t.Fatal("contact geometry changed", pose)
					}
				}
				area := (points[1][0]-points[0][0])*(points[2][1]-points[0][1]) - (points[1][1]-points[0][1])*(points[2][0]-points[0][0])
				if !patchFinite(area) || area <= 0 {
					t.Fatal("folded oral triangle", pose, ids, area)
				}
			})
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestMandibleLeavesLowerNeckStationary(t *testing.T) {
	root := os.Getenv("SYNTELLE_NATIVE_ASSETS")
	if root == "" {
		t.Skip("explicit private preparation fixture")
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
	r.prepareBody(FrameState{})
	base := append([]rasterVertex(nil), r.vertices...)
	chin := a.Motion.Anatomy.ChinY
	faceHeight := a.Motion.Anatomy.FaceHeight
	lowerNeck := chin + faceHeight*.20
	maximumNeckTravel := 0.0
	maximumChinTravel := 0.0
	for _, opening := range []float64{.25, .5, 1} {
		r.prepareBody(FrameState{JawOpen: opening})
		for i, v := range a.vertices {
			y := v.Y * float64(a.Motion.Canvas.Height)
			travel := math.Hypot(r.vertices[i].X-base[i].X, r.vertices[i].Y-base[i].Y)
			if y >= lowerNeck {
				maximumNeckTravel = math.Max(maximumNeckTravel, travel)
			}
			if math.Abs(y-chin) <= faceHeight*.03 {
				maximumChinTravel = math.Max(maximumChinTravel, travel)
			}
		}
	}
	if maximumNeckTravel > 1e-7 {
		t.Fatalf("jaw drags lower neck: %.4f px", maximumNeckTravel)
	}
	if maximumChinTravel < 3 {
		t.Fatalf("jaw no longer moves chin: %.4f px", maximumChinTravel)
	}
	t.Logf("jaw-only maximum chin travel %.4f px, lower neck %.8f px", maximumChinTravel, maximumNeckTravel)
}
