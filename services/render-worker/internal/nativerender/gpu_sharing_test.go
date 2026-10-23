//go:build integration && nativegpu && cgo && linux && amd64

package nativerender

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syntelle/render-worker/internal/nativeclock"
	"syscall"
	"testing"
	"unsafe"
)

// Real CUDA tests deliberately run serially: the accounting is process-wide.
func TestCUDASharedResources(t *testing.T) {
	if os.Getenv("SYNTELLE_GPU_CHECK") != "1" {
		t.Skip("explicit offline GPU qualification required")
	}
	t.Setenv("SYNTELLE_GPU_PIPELINE", "device")
	t.Setenv("SYNTELLE_GPU_SHARING", "on")
	assets, err := LoadCharacter(os.Getenv("SYNTELLE_NATIVE_ASSETS"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(os.Getenv("SYNTELLE_FRAME_STATES"))
	if err != nil {
		t.Fatal(err)
	}
	var states []struct {
		Name  string
		State FrameState
	}
	if err := json.Unmarshal(data, &states); err != nil {
		t.Fatal(err)
	}
	if len(states) < 19 {
		t.Fatal("recorded facial/motion states required")
	}
	var report []map[string]any
	var golden [4][]byte
	var frameHashes [4][19][32]byte
	for _, share := range []bool{false, true} {
		mode := "private"
		if share {
			mode = "shared"
		}
		t.Run(mode, func(t *testing.T) {
			if b, a, l := gpuMemory(); b != 0 || a != 0 || l != 0 {
				t.Fatal("prior leases survived", b, a, l)
			}
			renderers := make([]*Renderer, 4)
			frames := make([]*image.RGBA, 4)
			defer func() {
				for _, r := range renderers {
					if r != nil {
						if err := r.Close(); err != nil {
							t.Error(err)
						}
					}
				}
			}()
			start := nativeclock.Now()
			var checkpoints []map[string]any
			for i := range renderers {
				var memoryBefore, memoryAfter runtime.MemStats
				runtime.ReadMemStats(&memoryBefore)
				r, err := newRenderer(assets, "gpu")
				if err != nil {
					t.Fatal(err)
				}
				renderers[i] = r
				r.gpu.(*cudaRenderer).shareImmutable = share
				frames[i] = image.NewRGBA(image.Rect(0, 0, 1440, 960))
				if err := r.Render(context.Background(), states[i*3].State, frames[i]); err != nil {
					t.Fatal(err)
				}
				b, a, l := gpuMemory()
				var private uint64
				for _, other := range renderers {
					if other != nil {
						private += other.gpu.(*cudaRenderer).privateBytes()
					}
				}
				checkpoints = append(checkpoints, map[string]any{"sessions": i + 1, "immutable_bytes": b, "unique_allocations": a, "leases": l, "private_bytes": private, "total_renderer_device_bytes": b + private})
				runtime.ReadMemStats(&memoryAfter)
				hostAllocated := memoryAfter.TotalAlloc - memoryBefore.TotalAlloc
				checkpoints[len(checkpoints)-1]["host_initialization_allocated_bytes"] = hostAllocated
				// Subsequent scenes may allocate their working buffers, but must
				// not concatenate the full immutable atlas again on the host.
				if share && i > 0 && hostAllocated > max(uint64(64<<20), b/2) {
					t.Fatalf("shared scene rebuilt host materials: %d bytes", hostAllocated)
				}
				if share && i > 0 && checkpoints[0]["immutable_bytes"] != b {
					t.Fatal("immutable allocations grew for identical character")
				}
			}
			initializeMS := float64(nativeclock.Now()-start) / 1e6
			// Concurrent divergent conversations must not write each other's skin,
			// mouth, eyes, projected geometry or output. No device-wide serialization.
			start = nativeclock.Now()
			var usageBefore, usageAfter syscall.Rusage
			if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usageBefore); err != nil {
				t.Fatal(err)
			}
			var wg sync.WaitGroup
			errs := make(chan error, 4)
			for i, r := range renderers {
				wg.Go(func() {
					for n := 0; n < 19; n++ {
						if err := r.Render(context.Background(), states[(n+i*3)%len(states)].State, frames[i]); err != nil {
							errs <- err
							return
						}
						hash := sha256.Sum256(frames[i].Pix)
						if !share {
							frameHashes[i][n] = hash
						} else if frameHashes[i][n] != hash {
							errs <- fmt.Errorf("shared pixels differ: session %d state %d", i, n)
							return
						}
					}
				})
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				t.Fatal(err)
			}
			frameMS := float64(nativeclock.Now()-start) / 1e6 / 76
			if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usageAfter); err != nil {
				t.Fatal(err)
			}
			cpuMS := float64(usageAfter.Utime.Nano()+usageAfter.Stime.Nano()-usageBefore.Utime.Nano()-usageBefore.Stime.Nano()) / 1e6 / 76
			for i := range renderers {
				if !share {
					golden[i] = bytes.Clone(frames[i].Pix)
				} else if !bytes.Equal(golden[i], frames[i].Pix) {
					t.Fatal("shared pixels differ", i)
				}
			}
			// Release the first owner; all remaining leases must stay valid.
			if err := renderers[0].Close(); err != nil {
				t.Fatal(err)
			}
			renderers[0] = nil
			for i := 1; i < 4; i++ {
				if err := renderers[i].Render(context.Background(), states[(18+i*3)%len(states)].State, frames[i]); err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(golden[i], frames[i].Pix) {
					t.Fatal("owner release changed survivor pixels")
				}
			}
			for i, r := range renderers {
				if r != nil {
					if err := r.Close(); err != nil {
						t.Fatal(err)
					}
					renderers[i] = nil
				}
			}
			if b, a, l := gpuMemory(); b != 0 || a != 0 || l != 0 {
				t.Fatal("last close leaked immutable resources", b, a, l)
			}
			report = append(report, map[string]any{"mode": mode, "checkpoints": checkpoints, "initialization_ms": initializeMS, "render_wall_ms_per_frame": frameMS, "render_process_cpu_ms_per_frame": cpuMS, "concurrent_frames": 76, "pixels_exact": true, "last_close_released": true})
		})
	}
	t.Run("concurrent-cold-scenes", func(t *testing.T) {
		cold, err := LoadCharacter(os.Getenv("SYNTELLE_NATIVE_ASSETS"))
		if err != nil {
			t.Fatal(err)
		}
		var renderers [4]*Renderer
		defer func() {
			for _, r := range renderers {
				if r != nil {
					if err := r.Close(); err != nil {
						t.Error(err)
					}
				}
			}
		}()
		for i := range renderers {
			renderers[i], err = newRenderer(cold, "gpu")
			if err != nil {
				t.Fatal(err)
			}
		}
		var wg sync.WaitGroup
		errs := make(chan error, 4)
		for i, r := range renderers {
			wg.Go(func() {
				frame := image.NewRGBA(image.Rect(0, 0, 1440, 960))
				if err := r.Render(context.Background(), states[(18+i*3)%len(states)].State, frame); err != nil {
					errs <- err
					return
				}
				if !bytes.Equal(golden[i], frame.Pix) {
					errs <- fmt.Errorf("cold shared pixels differ: session %d", i)
				}
			})
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			t.Fatal(err)
		}
		if _, a, l := gpuMemory(); a != 11 || l != 44 {
			t.Fatal("cold concurrent scene sharing failed", a, l)
		}
	})
	// Content identity, not package/name identity: equal bytes share, changed
	// bytes stay distinct; rejected mutations leave the source intact.
	t.Run("content-and-write-protection", func(t *testing.T) {
		var gs []*cudaRenderer
		defer func() {
			for _, g := range gs {
				if err := g.Close(); err != nil {
					t.Error(err)
				}
			}
		}()
		a := []byte{1, 2, 3, 4}
		b := []byte{1, 2, 3, 5}
		for _, data := range [][]byte{a, a, b} {
			r, err := openGPU()
			if err != nil {
				t.Fatal(err)
			}
			g := r.(*cudaRenderer)
			gs = append(gs, g)
			if err := g.immutable(1, unsafe.Pointer(&data[0]), len(data)); err != nil {
				t.Fatal(err)
			}
		}
		if b, a, l := gpuMemory(); b != 8 || a != 2 || l != 3 {
			t.Fatal("incorrect content sharing", b, a, l)
		}
		if err := gs[0].upload(1, unsafe.Pointer(&b[0]), len(b)); err == nil {
			t.Fatal("immutable overwrite accepted")
		}
		if err := gs[0].immutable(0, unsafe.Pointer(&b[0]), len(b)); err == nil {
			t.Fatal("mutable subject accepted for sharing")
		}
		if err := gs[0].Close(); err != nil {
			t.Fatal(err)
		}
		if b, a, l := gpuMemory(); b != 8 || a != 2 || l != 2 {
			t.Fatal("live leases freed early", b, a, l)
		}
	})
	t.Run("concurrent-acquire-release", func(t *testing.T) {
		var wg sync.WaitGroup
		errs := make(chan error, 8)
		for range 8 {
			wg.Go(func() {
				for range 6 {
					r, err := openGPU()
					if err != nil {
						errs <- err
						return
					}
					g := r.(*cudaRenderer)
					data := []byte{9, 8, 7, 6}
					err = g.immutable(4, unsafe.Pointer(&data[0]), len(data))
					closeErr := g.Close()
					if err != nil || closeErr != nil {
						errs <- fmt.Errorf("lease lifecycle: %v / %v", err, closeErr)
						return
					}
				}
			})
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			t.Fatal(err)
		}
		if b, a, l := gpuMemory(); b != 0 || a != 0 || l != 0 {
			t.Fatal("concurrent leases leaked", b, a, l)
		}
	})
	if output := os.Getenv("SYNTELLE_SHARING_OUTPUT"); output != "" {
		encoded, err := json.MarshalIndent(map[string]any{"records": report, "content_isolation": true, "write_protection": true, "concurrent_lifecycles": 48, "paid_calls": 0}, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(output, "sharing.json"), encoded, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
