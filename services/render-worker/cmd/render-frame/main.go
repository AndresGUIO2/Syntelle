// Command render-frame replays fixed presentation states offline. It never
// connects to a provider or exposes the prepared package over HTTP.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"time"

	"syntelle/render-worker/internal/nativerender"
)

func run() error {
	var packageDir, states, output string
	var profile string
	var repeat int
	var components bool
	flag.StringVar(&packageDir, "package", "", "Private native package directory")
	flag.StringVar(&states, "states", "", "Recorded frame-state JSON")
	flag.StringVar(&output, "output", "", "New output directory")
	flag.StringVar(&profile, "cpuprofile", "", "Optional offline CPU profile")
	flag.IntVar(&repeat, "repeat", 1, "Number of offline state replays")
	flag.BoolVar(&components, "components", false, "Also export diagnostic mouth/eye patches")
	flag.Parse()
	if packageDir == "" || states == "" || output == "" {
		return fmt.Errorf("package, states and output are required")
	}
	assets, err := nativerender.Load(packageDir)
	if err != nil {
		return err
	}
	renderer, err := nativerender.New(assets)
	if err != nil {
		return err
	}
	bytes, err := os.ReadFile(states)
	if err != nil {
		return err
	}
	var cases []struct {
		Name  string
		State nativerender.FrameState
	}
	if err = json.Unmarshal(bytes, &cases); err != nil {
		return err
	}
	if err = os.MkdirAll(output, 0700); err != nil {
		return err
	}
	frame := image.NewRGBA(image.Rect(0, 0, 720, 960))
	mouth := nativerender.NewOralRenderer(assets.Oral)
	eye, err := nativerender.NewEyeRenderer(assets.Opened, assets.Closed, assets.EyeMask, assets.Sclera, assets.Ownership, assets.Motion.Eyes.Curves, assets.EyeSplit)
	if err != nil {
		return err
	}
	if profile != "" {
		f, e := os.Create(profile)
		if e != nil {
			return e
		}
		if e = pprof.StartCPUProfile(f); e != nil {
			f.Close()
			return e
		}
		defer f.Close()
		defer pprof.StopCPUProfile()
	}
	results := []any{}
	for iteration := 0; iteration < max(1, repeat); iteration++ {
		for _, c := range cases {
			if filepath.Base(c.Name) != c.Name || c.Name == "" {
				return fmt.Errorf("invalid fixture name")
			}
			start := time.Now()
			if err = renderer.Render(context.Background(), c.State, frame); err != nil {
				return err
			}
			elapsed := time.Since(start)
			file, e := os.Create(filepath.Join(output, c.Name+".png"))
			if e != nil {
				return e
			}
			e = png.Encode(file, frame)
			closeErr := file.Close()
			if e != nil {
				return e
			}
			if closeErr != nil {
				return closeErr
			}
			results = append(results, map[string]any{"name": c.Name, "render_ms": float64(elapsed) / float64(time.Millisecond)})
			if components {
				oral, e := mouth.Draw(c.State.From, c.State.To, c.State.Amount)
				if e != nil {
					return e
				}
				for name, t := range map[string]*nativerender.Texture{"mouth": oral, "eyes": eye.Draw(c.State.Closure, c.State.Gaze)} {
					file, e := os.Create(filepath.Join(output, c.Name+"-"+name+".png"))
					if e != nil {
						return e
					}
					var picture image.Image = &image.NRGBA{Pix: t.Pixels, Stride: t.Width * 4, Rect: image.Rect(0, 0, t.Width, t.Height)}
					if t.Premultiplied {
						picture = &image.RGBA{Pix: t.Pixels, Stride: t.Width * 4, Rect: image.Rect(0, 0, t.Width, t.Height)}
					}
					e = png.Encode(file, picture)
					closeErr := file.Close()
					if e != nil {
						return e
					}
					if closeErr != nil {
						return closeErr
					}
				}
			}
		}
	}
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	encoded, err := json.MarshalIndent(map[string]any{"cases": results, "heap_bytes": mem.HeapAlloc, "runtime": runtime.Version(), "provider_calls": 0}, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(output, "render.json"), append(encoded, '\n'), 0600); err != nil {
		return err
	}
	fmt.Println(string(encoded))
	return nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
