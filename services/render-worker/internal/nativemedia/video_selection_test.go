package nativemedia

import (
	"errors"
	"image"
	"syntelle/render-worker/internal/nativeframe"
	"testing"
)

type selectionEncoder struct {
	encodeErr, keyErr, closeErr error
	frames                      []*image.RGBA
	keys, closes                int
	packet                      []byte
}

type selectionSurfaceEncoder struct {
	selectionEncoder
	surfaceErr, errorEncode error
}

func (e *selectionSurfaceEncoder) Surface() (nativeframe.Surface, bool, error) {
	return nativeframe.Surface{}, e.surfaceErr == nil, e.surfaceErr
}
func (e *selectionSurfaceEncoder) EncodeSurface() ([]byte, error) {
	return []byte("obsolete GPU packet"), e.errorEncode
}

func TestDeviceFailureRequiresCurrentFrameReadbackAndFreshSoftwareIDR(t *testing.T) {
	for _, atSurface := range []bool{false, true} {
		t.Run(map[bool]string{false: "encode", true: "surface"}[atSurface], func(t *testing.T) {
			gpu := &selectionSurfaceEncoder{}
			failure := errors.New("device lost")
			if atSurface {
				gpu.surfaceErr = failure
			} else {
				gpu.errorEncode = failure
			}
			cpu := &selectionEncoder{packet: []byte("fresh SPS PPS IDR")}
			selected, err := selectVideoEncoder("auto", func() (VideoEncoder, error) { return gpu, nil }, func() (VideoEncoder, error) { return cpu, nil })
			if err != nil {
				t.Fatal(err)
			}
			v := selected.(*selectedVideo)
			_, available, err := v.Surface()
			if err != nil {
				t.Fatal(err)
			}
			if atSurface {
				if available {
					t.Fatal("failed surface remained available")
				}
			} else {
				if !available {
					t.Fatal("surface unavailable")
				}
				data, err := v.EncodeSurface()
				if len(data) != 0 || !errors.Is(err, errSurfaceRetry) {
					t.Fatal("partial device packet escaped")
				}
			}
			frame := image.NewRGBA(image.Rect(0, 0, 2, 2))
			data, err := v.Encode(frame)
			if err != nil || string(data) != "fresh SPS PPS IDR" {
				t.Fatal("fresh software frame missing")
			}
			if len(cpu.frames) != 1 || cpu.frames[0] != frame || cpu.keys != 1 || gpu.closes != 1 {
				t.Fatal("fallback ownership failed")
			}
			if err := v.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func (e *selectionEncoder) Encode(f *image.RGBA) ([]byte, error) {
	e.frames = append(e.frames, f)
	return e.packet, e.encodeErr
}
func (e *selectionEncoder) ForceKeyFrame() error { e.keys++; return e.keyErr }
func (e *selectionEncoder) Close() error         { e.closes++; return e.closeErr }

func TestVideoSelectionInitializationAndExplicitModes(t *testing.T) {
	for _, test := range []struct {
		mode                                string
		gpuFailure, wantHardware, wantError bool
		gpuCalls, cpuCalls                  int
	}{
		{"", false, true, false, 1, 0}, {"auto", true, false, false, 1, 1},
		{"openh264", false, false, false, 0, 1}, {"nvenc", true, false, true, 1, 0},
		{"nvenc", false, true, false, 1, 0}, {"unsupported", false, false, true, 0, 0},
	} {
		t.Run(test.mode, func(t *testing.T) {
			gpu, cpu := &selectionEncoder{}, &selectionEncoder{}
			gpuCalls, cpuCalls := 0, 0
			v, err := selectVideoEncoder(test.mode, func() (VideoEncoder, error) {
				gpuCalls++
				if test.gpuFailure {
					return nil, errors.New("no device / driver / capacity")
				}
				return gpu, nil
			}, func() (VideoEncoder, error) { cpuCalls++; return cpu, nil })
			if (err != nil) != test.wantError || gpuCalls != test.gpuCalls || cpuCalls != test.cpuCalls {
				t.Fatalf("selection err=%v calls=%d/%d", err, gpuCalls, cpuCalls)
			}
			if err == nil {
				if v.(*selectedVideo).hardware != test.wantHardware {
					t.Fatal("wrong backend")
				}
				if err := v.Close(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestVideoSelectionFailureReencodesSameFrameAsIDRWithoutRetryingGPU(t *testing.T) {
	gpu := &selectionEncoder{encodeErr: errors.New("device lost"), packet: []byte("partial stale GPU packet")}
	cpu := &selectionEncoder{packet: []byte("fresh SPS PPS IDR")}
	cpuCalls := 0
	v, err := selectVideoEncoder("auto", func() (VideoEncoder, error) { return gpu, nil },
		func() (VideoEncoder, error) { cpuCalls++; return cpu, nil })
	if err != nil {
		t.Fatal(err)
	}
	f := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for range 2 {
		packet, err := v.Encode(f)
		if err != nil || string(packet) != "fresh SPS PPS IDR" {
			t.Fatalf("packet=%s err=%v", packet, err)
		}
	}
	if gpu.closes != 1 || len(gpu.frames) != 1 || cpuCalls != 1 || cpu.keys != 1 || len(cpu.frames) != 2 || cpu.frames[0] != f {
		t.Fatal("fallback did not release GPU and retain only current frame")
	}
	if err := v.Close(); err != nil {
		t.Fatal(err)
	}
	if err := v.Close(); err != nil {
		t.Fatal(err)
	}
	if cpu.closes != 1 {
		t.Fatal("software encoder not closed exactly once")
	}
	if _, err := v.Encode(f); err == nil {
		t.Fatal("closed encoder accepted input")
	}
}

func TestVideoSelectionKeyframeFailureAndFallbackFailure(t *testing.T) {
	for _, failCPU := range []bool{false, true} {
		gpu := &selectionEncoder{keyErr: errors.New("GPU lost")}
		cpu := &selectionEncoder{keyErr: nil}
		if failCPU {
			cpu.keyErr = errors.New("software failure")
		}
		v, err := selectVideoEncoder("auto", func() (VideoEncoder, error) { return gpu, nil }, func() (VideoEncoder, error) { return cpu, nil })
		if err != nil {
			t.Fatal(err)
		}
		err = v.ForceKeyFrame()
		if (err != nil) != failCPU || gpu.closes != 1 || cpu.keys != 1 {
			t.Fatalf("fallback err=%v", err)
		}
		if err := v.Close(); err != nil {
			t.Fatal(err)
		}
		if cpu.closes != 1 {
			t.Fatal("failed or active software encoder leaked")
		}
	}
}

func TestRequiredNVENCDoesNotHideRuntimeFailure(t *testing.T) {
	failure := errors.New("device lost")
	gpu := &selectionEncoder{encodeErr: failure}
	v, err := selectVideoEncoder("nvenc", func() (VideoEncoder, error) { return gpu, nil }, func() (VideoEncoder, error) { t.Fatal("unexpected fallback"); return nil, nil })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Encode(nil); !errors.Is(err, failure) {
		t.Fatal("required mode hid GPU failure")
	}
	if err := v.Close(); err != nil {
		t.Fatal(err)
	}
}
