package nativemedia

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"time"

	"syntelle/render-worker/internal/nativeanimation"
)

const sampleRate = 24000
const audioSamples = sampleRate / 50
const maxQueuedParts = 32
const maxQueuedPCMBytes = 32 << 20

type preparedSpeech struct {
	Preview     bool            `json:"preview_only"`
	Audio       []byte          `json:"audio_base64"`
	DurationMS  float64         `json:"duration_ms"`
	Visemes     []Viseme        `json:"visemes"`
	Performance json.RawMessage `json:"performance"`
}

type speechPart struct {
	Speech   preparedSpeech `json:"speech"`
	Text     string         `json:"text"`
	Index    int            `json:"index"`
	Final    bool           `json:"final"`
	OffsetMS float64        `json:"offset_ms"`
}

type clip struct {
	preview     bool
	samples     []int16
	visemes     []Viseme
	duration    time.Duration
	text        string
	performance json.RawMessage
	// These fields are published under the session lock before audio can pop
	// the clip. Source slices and prepared analysis are immutable thereafter.
	needsPreparation bool
	prepared         *nativeanimation.PreparedSpeech
}

func decodeClip(prepared preparedSpeech, text string) (*clip, error) {
	samples, err := decodeWAV(prepared.Audio)
	if err != nil {
		return nil, err
	}
	duration := time.Duration(len(samples)) * time.Second / sampleRate
	if math.IsNaN(prepared.DurationMS) || math.IsInf(prepared.DurationMS, 0) || math.Abs(prepared.DurationMS-float64(duration)/float64(time.Millisecond)) > 2 {
		return nil, errors.New("speech duration does not match PCM")
	}
	previous := -1.0
	for _, v := range prepared.Visemes {
		if v.ID < 0 || v.ID > 21 || math.IsNaN(v.OffsetMS) || math.IsInf(v.OffsetMS, 0) || v.OffsetMS < 0 || v.OffsetMS < previous || v.OffsetMS > prepared.DurationMS+100 {
			return nil, errors.New("invalid native viseme timeline")
		}
		previous = v.OffsetMS
	}
	if len(prepared.Visemes) < 2 {
		return nil, errors.New("missing native viseme timeline")
	}
	return &clip{preview: prepared.Preview, samples: samples, visemes: prepared.Visemes, duration: duration, text: text, performance: prepared.Performance}, nil
}

func decodeWAV(data []byte) ([]int16, error) {
	if len(data) < 44 || len(data) > maxQueuedPCMBytes || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, errors.New("invalid bounded speech WAV")
	}
	if uint64(binary.LittleEndian.Uint32(data[4:8]))+8 > uint64(len(data)) {
		return nil, errors.New("truncated speech WAV")
	}
	var pcm []byte
	validFormat := false
	for at := 12; at+8 <= len(data); {
		size := uint64(binary.LittleEndian.Uint32(data[at+4 : at+8]))
		end := uint64(at) + 8 + size
		if end > uint64(len(data)) {
			return nil, errors.New("truncated speech WAV chunk")
		}
		chunk := data[at+8 : int(end)]
		switch string(data[at : at+4]) {
		case "fmt ":
			validFormat = len(chunk) >= 16 && binary.LittleEndian.Uint16(chunk) == 1 && binary.LittleEndian.Uint16(chunk[2:]) == 1 && binary.LittleEndian.Uint32(chunk[4:]) == sampleRate && binary.LittleEndian.Uint16(chunk[14:]) == 16
		case "data":
			pcm = chunk
		}
		at = int(end) + (int(size) & 1)
	}
	if !validFormat || len(pcm) == 0 || len(pcm)%2 != 0 {
		return nil, errors.New("expected mono PCM16 24 kHz speech")
	}
	result := make([]int16, len(pcm)/2)
	for i := range result {
		result[i] = int16(binary.LittleEndian.Uint16(pcm[i*2:]))
	}
	return result, nil
}
