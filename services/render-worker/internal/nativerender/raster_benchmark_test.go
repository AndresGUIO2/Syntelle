package nativerender

import (
	"context"
	"image"
	"os"
	"testing"
)

// Set SYNTELLE_NATIVE_ASSETS to the existing exported immutable package. The
// benchmark never prepares assets, contacts a provider, or writes image files.
func BenchmarkPreparedRendererFrame(b *testing.B) {
	directory := os.Getenv("SYNTELLE_NATIVE_ASSETS")
	if directory == "" {
		b.Skip("set SYNTELLE_NATIVE_ASSETS to a prepared native package")
	}
	assets, err := Load(directory)
	if err != nil {
		b.Fatal(err)
	}
	for _, test := range []struct {
		name     string
		state    FrameState
		animated bool
	}{
		{"idle", FrameState{Head: [3]float64{1.1, -.4, .1}, Breath: 1.3, From: "Rest", To: "A", Background: "dark-lounge"}, false},
		{"speech", FrameState{Head: [3]float64{1.1, -.4, .1}, Breath: 1.3, From: "A", To: "FV", Amount: .5, Closure: .3, Background: "dark-lounge"}, false},
		{"smile", FrameState{Head: [3]float64{1.1, -.4, .1}, Breath: 1.3, From: "Rest", To: "A", Smile: 1, Background: "dark-lounge"}, false},
		{"speech-animated", FrameState{Head: [3]float64{1.1, -.4, .1}, Breath: 1.3, From: "A", To: "FV", Amount: .5, Closure: .3, Background: "dark-lounge"}, true},
	} {
		b.Run(test.name, func(b *testing.B) {
			renderer, err := New(assets)
			if err != nil {
				b.Fatal(err)
			}
			b.Cleanup(func() {
				if err := renderer.Close(); err != nil {
					b.Error(err)
				}
			})
			frame := image.NewRGBA(image.Rect(0, 0, 720, 960))
			ctx := context.Background()
			if err := renderer.Render(ctx, test.state, frame); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			frameIndex := 0
			for b.Loop() {
				if test.animated {
					test.state.Amount = float64(frameIndex%3+1) / 4
					test.state.Head[0] = 1.1 + float64(frameIndex%5)*.17
					frameIndex++
				}
				if err := renderer.Render(ctx, test.state, frame); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
