package nativerender

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
)

func TestPreparedJointExpressionsPreserveNeutralAndImmutableMaterials(t *testing.T) {
	directory := os.Getenv("SYNTELLE_JOINT_TEST_ASSETS")
	if directory == "" {
		directory = filepath.Join("..", "..", "..", "..", ".tools", "expressive-performance-67", "runtime-package")
	}
	if _, err := os.Stat(filepath.Join(directory, "manifest.json")); os.IsNotExist(err) {
		t.Skip("private prepared expression fixture unavailable")
	}
	a, err := Load(directory)
	if err != nil {
		t.Fatal(err)
	}
	r, err := newRenderer(a, "cpu")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	upper, lower := sha256.Sum256(a.Oral.upper.Pixels), sha256.Sum256(a.Oral.lower.Pixels)
	foreground := sha256.Sum256(a.Foreground.Pixels)
	plain := NewOralRenderer(a.Oral)
	joint := NewOralRenderer(a.Oral)
	joint.joint = a.Expression.Joint
	for _, pose := range []string{"Rest", "A", "EI", "O", "U", "FV", "SZ", "MBP", "CHSHR"} {
		to := "A"
		if pose == "A" {
			to = "Rest"
		}
		original, err := plain.Draw(pose, to, 0)
		if err != nil {
			t.Fatal(err)
		}
		neutral, err := joint.DrawExpression(pose, to, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(original.Pixels, neutral.Pixels) {
			t.Fatalf("neutral changed for %s", pose)
		}
		for _, smile := range []float64{.25, .5, 1} {
			for _, laugh := range []float64{0, .5, 1} {
				joint.laugh = laugh
				if _, err = joint.DrawExpression(pose, to, 0, smile); err != nil {
					t.Fatal(pose, smile, laugh, err)
				}
				r.prepareSkin(smile, laugh)
			}
		}
	}
	r.prepareSkin(0, 0)
	if !bytes.Equal(r.expressionBase.Pixels, a.Foreground.Pixels) {
		t.Fatal("neutral skin is not source-exact")
	}
	r.prepareSkin(1, 1)
	crop := a.Expression.Crop
	for y := range a.Foreground.Height {
		for x := range a.Foreground.Width {
			i := (y*a.Foreground.Width + x) * 4
			if r.expressionBase.Pixels[i+3] != a.Foreground.Pixels[i+3] {
				t.Fatal("foreground alpha changed")
			}
			if (x < crop.X || x >= crop.X+crop.Width || y < crop.Y || y >= crop.Y+crop.Height) && !bytes.Equal(r.expressionBase.Pixels[i:i+4], a.Foreground.Pixels[i:i+4]) {
				t.Fatal("protected pixel changed")
			}
		}
	}
	if upper != sha256.Sum256(a.Oral.upper.Pixels) || lower != sha256.Sum256(a.Oral.lower.Pixels) || foreground != sha256.Sum256(a.Foreground.Pixels) {
		t.Fatal("immutable source material changed")
	}
}
