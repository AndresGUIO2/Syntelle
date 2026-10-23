//go:build nativecodecs && cgo

package nativemedia

import (
	"bytes"
	"encoding/binary"
	"image"
	"testing"

	"github.com/pion/rtp/codecs"
)

func codecOwnershipFrame(width, height int) *image.RGBA {
	frame := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			i := y*frame.Stride + x*4
			frame.Pix[i] = byte(31 + x*7 + y*3)
			frame.Pix[i+1] = byte(23 + y*5)
			frame.Pix[i+2] = byte(41 + x*3)
			frame.Pix[i+3] = 255
		}
	}
	return frame
}

func nativeNAL(data []byte, kind byte) []byte {
	_, begin := annexStart(data, 0)
	for begin >= 0 && begin < len(data) {
		next, nextBegin := annexStart(data, begin)
		end := next
		if end < 0 {
			end = len(data)
		}
		if data[begin]&31 == kind {
			return data[begin:end]
		}
		begin = nextBegin
	}
	return nil
}

// Pion may retain SPS/PPS between Payload calls. Every encoded access unit
// must remain Go-owned after codec release, subsequent Encode and Close.
func TestNativeEncodedBuffersSurviveLaterReadsAndClose(t *testing.T) {
	factory, err := defaultEncoders()
	if err != nil {
		t.Fatal(err)
	}
	video, err := factory.Video(128, 128, 30, 2500000)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = video.Close() })
	frame := codecOwnershipFrame(128, 128)
	key, err := video.Encode(frame)
	if err != nil {
		t.Fatal(err)
	}
	sps, pps := nativeNAL(key, 7), nativeNAL(key, 8)
	if len(sps) == 0 || len(pps) == 0 {
		t.Fatal("native keyframe must supply SPS and PPS")
	}
	wantKey, wantSPS, wantPPS := bytes.Clone(key), bytes.Clone(sps), bytes.Clone(pps)
	payloader := &codecs.H264Payloader{}
	if len(payloader.Payload(1200, sps)) != 0 || len(payloader.Payload(1200, pps)) != 0 {
		t.Fatal("pinned payloader did not retain separate SPS/PPS")
	}
	inter, err := video.Encode(frame)
	if err != nil {
		t.Fatal(err)
	}
	if len(inter) == 0 || nativeNAL(inter, 7) != nil {
		t.Fatal("expected a native interframe without SPS color-copy allocation")
	}
	wantInter := bytes.Clone(inter)
	for i := range 4 {
		frame.Pix[(i+1)*frame.Stride+4] ^= 127
		if err := video.ForceKeyFrame(); err != nil {
			t.Fatal(err)
		}
		if _, err := video.Encode(frame); err != nil {
			t.Fatal(err)
		}
	}
	if err := video.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(key, wantKey) || !bytes.Equal(inter, wantInter) {
		t.Fatal("a subsequent encode or codec close mutated an earlier access unit")
	}
	payloads := payloader.Payload(1200, []byte{0x65, 0x40, 0x80})
	if len(payloads) != 2 || payloads[0][0]&31 != 24 {
		t.Fatal("retained SPS/PPS did not produce a STAP-A packet")
	}
	stap := payloads[0][1:]
	for _, want := range [][]byte{wantSPS, wantPPS} {
		if len(stap) < 2 {
			t.Fatal("truncated retained STAP-A")
		}
		n := int(binary.BigEndian.Uint16(stap))
		stap = stap[2:]
		if n > len(stap) || !bytes.Equal(stap[:n], want) {
			t.Fatal("retained parameter set changed after codec release")
		}
		stap = stap[n:]
	}
	if len(stap) != 0 {
		t.Fatal("unexpected retained parameter bytes")
	}

	audio, err := factory.Audio(sampleRate, 1, 32000)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = audio.Close() })
	pcm := make([]int16, audioSamples)
	for i := range pcm {
		pcm[i] = int16((i%48)*500 - 12000)
	}
	packet, err := audio.Encode(pcm)
	if err != nil {
		t.Fatal(err)
	}
	if len(packet) < 3 {
		t.Fatal("native Opus packet was empty")
	}
	wantPacket := bytes.Clone(packet)
	for range 4 {
		pcm[0]++
		if _, err := audio.Encode(pcm); err != nil {
			t.Fatal(err)
		}
	}
	if err := audio.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(packet, wantPacket) {
		t.Fatal("a subsequent encode or codec close mutated an earlier Opus packet")
	}
}

var encodedCopySink []byte

// Compare the real pinned native encoding stage with one extra full-payload
// Go copy representing the former step. Upstream allocations remain in both.
func BenchmarkNativeEncodedCopy(b *testing.B) {
	for _, copyOutput := range []bool{false, true} {
		name := "owned-output"
		if copyOutput {
			name = "previous-extra-copy"
		}
		b.Run("H264/"+name, func(b *testing.B) {
			factory, err := defaultEncoders()
			if err != nil {
				b.Fatal(err)
			}
			encoder, err := factory.Video(720, 960, 30, 2500000)
			if err != nil {
				b.Fatal(err)
			}
			defer func() { _ = encoder.Close() }()
			frame := codecOwnershipFrame(720, 960)
			b.ReportAllocs()
			b.ResetTimer()
			var payloadBytes int64
			for i := range b.N {
				frame.Pix[((i%60)+400)*frame.Stride+360*4] ^= 127
				data, err := encoder.Encode(frame)
				if err != nil {
					b.Fatal(err)
				}
				if copyOutput {
					data = append([]byte(nil), data...)
				}
				payloadBytes += int64(len(data))
				encodedCopySink = data
			}
			b.ReportMetric(float64(payloadBytes)/float64(b.N), "payload-B/op")
		})
		b.Run("Opus/"+name, func(b *testing.B) {
			factory, err := defaultEncoders()
			if err != nil {
				b.Fatal(err)
			}
			encoder, err := factory.Audio(sampleRate, 1, 32000)
			if err != nil {
				b.Fatal(err)
			}
			defer func() { _ = encoder.Close() }()
			pcm := make([]int16, audioSamples)
			for i := range pcm {
				pcm[i] = int16((i%48)*500 - 12000)
			}
			b.ReportAllocs()
			b.ResetTimer()
			var payloadBytes int64
			for range b.N {
				data, err := encoder.Encode(pcm)
				if err != nil {
					b.Fatal(err)
				}
				if copyOutput {
					data = append([]byte(nil), data...)
				}
				payloadBytes += int64(len(data))
				encodedCopySink = data
			}
			b.ReportMetric(float64(payloadBytes)/float64(b.N), "payload-B/op")
		})
	}
}
