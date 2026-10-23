package nativerender

import (
	"bytes"
	"encoding/json"
	"testing"
)

func patchSolid(width, height int, pixel [4]byte) *Texture {
	image := NewTexture(width, height, false)
	for i := 0; i < len(image.Pixels); i += 4 {
		copy(image.Pixels[i:i+4], pixel[:])
	}
	return image
}

func patchOralFixture(t *testing.T) (*OralAssets, map[string]*Texture) {
	t.Helper()
	const width, height = 4, 4
	points := make([][2]float64, 40)
	copy(points, [][2]float64{{0, 0}, {width, 0}, {width, height}, {0, height}})
	for i := 20; i < 40; i++ {
		points[i] = [2]float64{1, 1}
	}
	points[30], points[35] = [2]float64{3, 1}, [2]float64{1, 3}
	mesh := &oralMesh{Width: width, Height: height, Rest: points, Open: points, Triangles: [][3]int{{0, 1, 2}, {0, 2, 3}}}
	resources := map[string]*Texture{
		"from":      patchSolid(width, height, [4]byte{20, 40, 60, 255}),
		"to":        patchSolid(width, height, [4]byte{100, 120, 140, 255}),
		"mask-from": patchSolid(width, height, [4]byte{128, 0, 0, 255}),
		"mask-to":   patchSolid(width, height, [4]byte{64, 0, 0, 255}),
		"upper":     patchSolid(width, height, [4]byte{}),
		"lower":     patchSolid(width, height, [4]byte{}),
		"cavity":    patchSolid(width, height, [4]byte{0, 0, 0, 255}),
	}
	definition := struct {
		Crop      map[string]int
		Reference map[string]string
		Oral      oralLighting
		Poses     map[string]oralDefinition
		Pairs     map[string]oralPair
	}{Crop: map[string]int{"width": width, "height": height}, Reference: map[string]string{"upper": "upper", "lower": "lower"},
		Oral: oralLighting{ShadowWidth: 1},
		Poses: map[string]oralDefinition{
			"Rest": {ID: "Rest", Surface: "from", Mask: "mask-from", Cavity: "cavity", ApertureSDF: "source-bound", LowerTranslation: []float64{0, 0}},
			"A":    {ID: "A", Surface: "to", Mask: "mask-to", Cavity: "cavity", ApertureSDF: "source-bound", LowerTranslation: []float64{0, 0}},
		},
		Pairs: map[string]oralPair{"Rest-A": {Safe: true, Mesh: mesh}, "A-Rest": {Safe: true, Mesh: mesh}},
	}
	data, err := json.Marshal(definition)
	if err != nil {
		t.Fatal(err)
	}
	assets, err := NewOralAssets(data, func(name string) (*Texture, error) { return resources[name], nil })
	if err != nil {
		t.Fatal(err)
	}
	return assets, resources
}

func TestOralRendererPreservesEndpointsAndUnwarpedAlpha(t *testing.T) {
	t.Parallel()
	assets, _ := patchOralFixture(t)
	renderer := NewOralRenderer(assets)
	for _, test := range []struct {
		name   string
		amount float64
		pixel  [4]byte
	}{{"source", 0, [4]byte{20, 40, 60, 128}}, {"middle", .5, [4]byte{60, 80, 100, 96}}, {"target", 1, [4]byte{100, 120, 140, 64}}} {
		t.Run(test.name, func(t *testing.T) {
			output, err := renderer.Draw("Rest", "A", test.amount)
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < len(output.Pixels); i += 4 {
				if !bytes.Equal(output.Pixels[i:i+4], test.pixel[:]) {
					t.Fatalf("pixel %d = %v; want %v", i/4, output.Pixels[i:i+4], test.pixel)
				}
			}
		})
	}
	if _, err := renderer.Draw("unknown", "A", .5); err == nil {
		t.Fatal("unknown pose was accepted")
	}
	assets.pairs["Rest-A"] = oralPair{Safe: false}
	if _, err := NewOralRenderer(assets).Draw("Rest", "A", .5); err == nil {
		t.Fatal("unsafe transition was accepted")
	}
}

func TestOralRendererFiltersCoveredDentalColor(t *testing.T) {
	t.Parallel()
	assets, resources := patchOralFixture(t)
	upper := patchSolid(4, 4, [4]byte{200, 100, 50, 128})
	original := append([]byte(nil), upper.Pixels...)
	assets.upper = patchPremultipliedCopy(upper)
	for _, pose := range assets.poses {
		for i := 0; i < len(pose.mask.Pixels); i += 4 {
			pose.mask.Pixels[i], pose.mask.Pixels[i+1] = 255, 255
		}
	}
	output, err := NewOralRenderer(assets).Draw("Rest", "A", .5)
	if err != nil {
		t.Fatal(err)
	}
	// Transparent edge texels contain covered RGB. The shader unpremultiplies
	// once before alpha composition, yielding the covered RGB exactly once.
	if got, want := output.Pixels[4*5:4*6], []byte{100, 50, 25, 255}; !bytes.Equal(got, want) {
		t.Fatalf("dental edge = %v; want %v", got, want)
	}
	if !bytes.Equal(upper.Pixels, original) || resources["upper"].Premultiplied {
		t.Fatal("rendering changed an immutable material")
	}
}

func TestEyeRendererKeepsNeutralAndClosedSourcePixels(t *testing.T) {
	t.Parallel()
	opened := patchSolid(4, 3, [4]byte{20, 40, 60, 255})
	closed := patchSolid(4, 3, [4]byte{100, 120, 140, 255})
	mask := patchSolid(4, 3, [4]byte{128, 0, 0, 255})
	clean := patchSolid(4, 3, [4]byte{255, 255, 255, 255})
	ownership := patchSolid(4, 3, [4]byte{0, 0, 0, 255})
	curves := [][]float64{{0, 2, 1, 0}, {0, 2, 1, 0}, {0, 2, 1, 0}, {0, 2, 1, 0}}
	renderer, err := NewEyeRenderer(opened, closed, mask, clean, ownership, curves, 2)
	if err != nil {
		t.Fatal(err)
	}
	if output := renderer.Draw(0, [2][2]float64{}); !bytes.Equal(output.Pixels, opened.Pixels) {
		t.Fatal("neutral eye changed source pixels")
	}
	closedOutput := renderer.Draw(1, [2][2]float64{{4, 1}, {-4, -1}})
	if got, want := closedOutput.Pixels[:4], []byte{60, 80, 100, 255}; !bytes.Equal(got, want) {
		t.Fatalf("closed eye = %v; want %v", got, want)
	}
}

func TestEyeRendererRevealsScleraAtVacatedIris(t *testing.T) {
	t.Parallel()
	opened := patchSolid(5, 3, [4]byte{255, 255, 255, 255})
	closed := patchSolid(5, 3, [4]byte{255, 255, 255, 255})
	clean := patchSolid(5, 3, [4]byte{255, 255, 255, 255})
	mask := patchSolid(5, 3, [4]byte{255, 0, 0, 255})
	ownership := patchSolid(5, 3, [4]byte{255, 0, 255, 255})
	for y := range 3 {
		i := (y*5 + 2) * 4
		opened.Pixels[i], opened.Pixels[i+1], opened.Pixels[i+2] = 0, 0, 0
		ownership.Pixels[i+1] = 255
	}
	curves := make([][]float64, 5)
	for x := range curves {
		curves[x] = []float64{0, 2, 1, 0}
	}
	renderer, err := NewEyeRenderer(opened, closed, mask, clean, ownership, curves, 5)
	if err != nil {
		t.Fatal(err)
	}
	output := renderer.Draw(0, [2][2]float64{{1, 0}, {1, 0}})
	if got := output.Pixels[(5+2)*4 : (5+3)*4]; !bytes.Equal(got, []byte{255, 255, 255, 255}) {
		t.Fatalf("vacated iris did not reveal sclera: %v", got)
	}
	if got := output.Pixels[(5+3)*4 : (5+4)*4]; !bytes.Equal(got, []byte{0, 0, 0, 255}) {
		t.Fatalf("translated iris was not retained: %v", got)
	}
	if _, err := NewEyeRenderer(opened, closed, mask, clean, ownership, curves[:4], 5); err == nil {
		t.Fatal("incomplete eyelid curves were accepted")
	}
}
