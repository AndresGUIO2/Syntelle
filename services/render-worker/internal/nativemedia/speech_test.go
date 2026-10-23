package nativemedia

import (
	"encoding/binary"
	"testing"
)

func testWAV(samples int) []byte {
	wav := make([]byte, 44+samples*2)
	copy(wav, "RIFF")
	binary.LittleEndian.PutUint32(wav[4:], uint32(len(wav)-8))
	copy(wav[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(wav[16:], 16)
	binary.LittleEndian.PutUint16(wav[20:], 1)
	binary.LittleEndian.PutUint16(wav[22:], 1)
	binary.LittleEndian.PutUint32(wav[24:], 24000)
	binary.LittleEndian.PutUint32(wav[28:], 48000)
	binary.LittleEndian.PutUint16(wav[32:], 2)
	binary.LittleEndian.PutUint16(wav[34:], 16)
	copy(wav[36:], "data")
	binary.LittleEndian.PutUint32(wav[40:], uint32(samples*2))
	for i := 0; i < samples; i++ {
		binary.LittleEndian.PutUint16(wav[44+i*2:], uint16(i%2000))
	}
	return wav
}

func TestDecodeWAV(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name   string
		change func([]byte) []byte
		valid  bool
	}{
		{name: "native mono PCM", change: func(v []byte) []byte { return v }, valid: true},
		{name: "truncated chunk", change: func(v []byte) []byte { return v[:len(v)-1] }},
		{name: "stereo rejected", change: func(v []byte) []byte { binary.LittleEndian.PutUint16(v[22:], 2); return v }},
		{name: "wrong sample rate", change: func(v []byte) []byte { binary.LittleEndian.PutUint32(v[24:], 48000); return v }},
		{name: "compressed format rejected", change: func(v []byte) []byte { binary.LittleEndian.PutUint16(v[20:], 3); return v }},
		{name: "odd sample bytes rejected", change: func(v []byte) []byte { binary.LittleEndian.PutUint32(v[40:], 959); return v }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			samples, err := decodeWAV(tt.change(testWAV(480)))
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%v error=%v", tt.valid, err)
			}
			if tt.valid && (len(samples) != 480 || samples[123] != 123) {
				t.Fatalf("PCM samples changed")
			}
		})
	}
}

func TestDecodeClipRejectsInvalidNativeTimeline(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name     string
		duration float64
		visemes  []Viseme
	}{
		{name: "missing visemes", duration: 20},
		{name: "invalid Azure id", duration: 20, visemes: []Viseme{{ID: 22}, {ID: 0, OffsetMS: 20}}},
		{name: "out of order", duration: 20, visemes: []Viseme{{ID: 1, OffsetMS: 10}, {ID: 0, OffsetMS: 0}}},
		{name: "PCM duration mismatch", duration: 200, visemes: []Viseme{{ID: 1}, {ID: 0, OffsetMS: 20}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := decodeClip(preparedSpeech{Audio: testWAV(480), DurationMS: tt.duration, Visemes: tt.visemes}, "fixture"); err == nil {
				t.Fatal("invalid provider fixture was accepted")
			}
		})
	}
}
