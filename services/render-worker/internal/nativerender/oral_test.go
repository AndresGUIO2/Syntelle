package nativerender

import (
	"encoding/json"
	"fmt"
	"testing"

	"syntelle/render-contract"
)

func TestEmotionWithoutSmileTransportsLipTextureAndCoverage(t *testing.T) {
	for _, amount := range []float64{0, .5, 1} {
		t.Run(fmt.Sprintf("articulation-%g", amount), func(t *testing.T) {
			assets, _ := patchOralFixture(t)
			for _, pose := range assets.poses {
				for y := range 4 {
					for x := range 4 {
						at := (y*4 + x) * 4
						pose.mask.Pixels[at] = 0
						pose.surface.Pixels[at] = 150
						if x == 0 {
							pose.mask.Pixels[at] = 255
							pose.surface.Pixels[at] = 24
						}
					}
				}
			}
			delta := make([][2]float64, 40)
			for i := range delta {
				delta[i][0] = 1
			}
			data, err := json.Marshal(map[string]any{
				"edit_coverage": "deformed-oral-coverage/1",
				"oral": map[string]any{"states": map[string]map[string]JointPair{
					"sad": {"Rest-A": {RestDelta: delta, OpenDelta: delta}},
				}},
			})
			if err != nil {
				t.Fatal(err)
			}
			rig := &EmotionRig{}
			if err := json.Unmarshal(data, rig); err != nil {
				t.Fatal(err)
			}
			renderer := NewOralRenderer(assets)
			renderer.emotionRig = rig
			renderer.emotion[rendercontract.EmotionIndex("sad")] = 1
			output, err := renderer.DrawExpression("Rest", "A", amount, 0)
			if err != nil {
				t.Fatal(err)
			}
			at := (1*4 + 1) * 4
			if output.Pixels[at] != 24 || output.Pixels[at+3] != 255 {
				t.Fatalf("moved source lip is clipped or samples destination skin: %v", output.Pixels[at:at+4])
			}
			if output.Pixels[(1*4+2)*4+3] != 0 {
				t.Fatal("transported coverage leaks outside source ownership")
			}
		})
	}
}
