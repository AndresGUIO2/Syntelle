//go:build nativecodecs && cgo

package nativemedia

import (
	"image"
	"testing"
)

func TestNativeCodecsEncodeRealH264AndOpus(t *testing.T) {
	factory, err := defaultEncoders()
	if err != nil {
		t.Fatal(err)
	}
	video, err := factory.Video(720, 960, 30, 2500000)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := video.Close(); err != nil {
			t.Error(err)
		}
	}()
	frame := image.NewRGBA(image.Rect(0, 0, 720, 960))
	for i := 0; i < len(frame.Pix); i += 4 {
		frame.Pix[i] = 120
		frame.Pix[i+1] = 80
		frame.Pix[i+2] = 170
		frame.Pix[i+3] = 255
	}
	if err := video.ForceKeyFrame(); err != nil {
		t.Fatal(err)
	}
	encoded, err := video.Encode(frame)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) < 32 || encoded[0] != 0 || encoded[1] != 0 {
		t.Fatalf("not an H264 access unit: %d bytes", len(encoded))
	}
	_, nal := annexStart(encoded, 0)
	end, _ := annexStart(encoded, nal)
	if nal < 0 || end < 0 || encoded[nal]&31 != 7 {
		t.Fatal("first native access unit did not include SPS")
	}
	full, colors, _ := inspectSPSColor(t, encoded[nal:end])
	if full || colors != [3]uint32{1, 1, 6} {
		t.Fatalf("encoder color metadata does not match I420 input: %v %v", full, colors)
	}
	audio, err := factory.Audio(sampleRate, 1, 32000)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := audio.Close(); err != nil {
			t.Error(err)
		}
	}()
	pcm := make([]int16, audioSamples)
	for i := range pcm {
		pcm[i] = int16((i%48)*500 - 12000)
	}
	packet, err := audio.Encode(pcm)
	if err != nil {
		t.Fatal(err)
	}
	if len(packet) < 3 {
		t.Fatal("Opus encoder returned no usable packet")
	}
}
