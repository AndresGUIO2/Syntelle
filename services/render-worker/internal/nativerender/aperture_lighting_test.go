package nativerender

import (
	"bytes"
	"testing"
)

func TestVisibleApertureShadowsNarrowEnamelWithoutChangingCoverage(t *testing.T) {
	a, _ := patchOralFixture(t)
	a.upper = patchSolid(4, 4, [4]byte{240, 225, 210, 255})
	original := append([]byte(nil), a.upper.Pixels...)
	a.lighting.ShadowStrength = .3
	a.lighting.Visible = &apertureLighting{ReferenceGap: 60, Minimum: .28, FullRatio: .65, ContactWidth: 7.2}
	render := func(distance, ratio byte) []byte {
		for _, p := range a.poses {
			for i := 0; i < len(p.mask.Pixels); i += 4 {
				p.mask.Pixels[i] = 255
				p.mask.Pixels[i+1] = distance
				p.mask.Pixels[i+2] = ratio
			}
		}
		out, err := NewOralRenderer(a).Draw("Rest", "A", .5)
		if err != nil {
			t.Fatal(err)
		}
		return append([]byte(nil), out.Pixels...)
	}
	narrow := render(136, 13)
	wide := render(240, 255)
	if narrow[0] >= 170 || wide[0] <= 210 {
		t.Fatalf("insufficient closure shading: narrow=%v wide=%v", narrow[:4], wide[:4])
	}
	for i := 3; i < len(narrow); i += 4 {
		if narrow[i] != wide[i] {
			t.Fatal("lighting changed opacity")
		}
	}
	if !bytes.Equal(a.upper.Pixels, original) {
		t.Fatal("canonical enamel changed")
	}
}
