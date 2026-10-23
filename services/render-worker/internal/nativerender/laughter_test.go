package nativerender

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
)

func TestBoundaryLaughterRejectsSpeechPosesAndPreservesDentition(t *testing.T) {
	directory := filepath.Join("..", "..", "..", "..", ".tools", "laughter-face-74", "package-v6")
	if _, err := os.Stat(directory); err != nil {
		t.Skip("local laughter package unavailable")
	}
	a, err := Load(directory)
	if err != nil {
		t.Fatal(err)
	}
	r, err := newRenderer(a, "cpu")
	if err != nil {
		t.Fatal(err)
	}
	upper, lower := sha256.Sum256(a.Oral.upper.Pixels), sha256.Sum256(a.Oral.lower.Pixels)
	r.oral.laugh = 1
	if _, err := r.oral.DrawExpression("Rest", "A", .06, 1); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, from, to string
		amount         float64
	}{
		{"speech articulation", "FV", "A", .02}, {"large jaw release", "Rest", "A", .2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := r.oral.DrawExpression(tc.from, tc.to, tc.amount, 1); err == nil {
				t.Fatal("unsupported boundary movement admitted")
			}
		})
	}
	if upper != sha256.Sum256(a.Oral.upper.Pixels) || lower != sha256.Sum256(a.Oral.lower.Pixels) {
		t.Fatal("canonical dentition changed")
	}
	if r.laughSquint(FrameState{Laugh: 1}) <= 0 || r.laughSquint(FrameState{}) != 0 {
		t.Fatal("lower eyelid expression lost its prepared bounds")
	}
}
