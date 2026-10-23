package nativeanimation

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"
)

type referenceFixture struct {
	Bundle struct{ Motion, Gaze, Expressions, Oral json.RawMessage }
	Speech struct {
		SampleRate     int    `json:"sample_rate"`
		AudioBase64    string `json:"audio_base64"`
		Visemes        []NativeViseme
		Performance    *Performance
		Plan           *Plan
		AutonomousPlan *Plan `json:"autonomous_plan"`
		Timeline       []TimelineEvent
	}
	Cases []struct {
		Name string
		Rows []struct {
			MS       float64
			Settings Settings
			Speech   struct {
				Active   bool
				Waiting  bool
				Position float64
				Session  uint64
				HasPlan  bool `json:"has_plan"`
			}
			Expected Frame
		}
	}
}

func loadReference(t *testing.T) (referenceFixture, Config, []float32) {
	t.Helper()
	raw, err := os.ReadFile("testdata/animation-fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var ref referenceFixture
	if err = json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	config, err := LoadBundle(ref.Bundle.Motion, ref.Bundle.Gaze, ref.Bundle.Expressions, ref.Bundle.Oral)
	if err != nil {
		t.Fatal(err)
	}
	config.Expressive = false
	wav, err := base64.StdEncoding.DecodeString(ref.Speech.AudioBase64)
	if err != nil {
		t.Fatal(err)
	}
	var samples []float32
	for at := 12; at+8 <= len(wav); {
		size := int(binary.LittleEndian.Uint32(wav[at+4:]))
		end := at + 8 + size
		if end > len(wav) {
			t.Fatal("invalid cached WAV fixture")
		}
		if string(wav[at:at+4]) == "data" {
			samples = make([]float32, size/2)
			for i := range samples {
				samples[i] = float32(int16(binary.LittleEndian.Uint16(wav[at+8+i*2:]))) / 32768
			}
		}
		at = end + size%2
	}
	if len(samples) == 0 {
		t.Fatal("cached PCM missing")
	}
	return ref, config, samples
}
func frameValues(f Frame) []float64 {
	return []float64{f.Head.Yaw, f.Head.Pitch, f.Head.Roll, f.Breath, f.Mouth.Amount, f.EyeClosure, f.Gaze[0][0], f.Gaze[0][1], f.Gaze[1][0], f.Gaze[1][1], f.Smile, f.Brows}
}
func TestEngineSampleMatchesOriginalTypeScript(t *testing.T) {
	t.Parallel()
	ref, c, samples := loadReference(t)
	speech, err := BuildSpeechWithPerformance(samples, ref.Speech.SampleRate, ref.Speech.Visemes, 123456789, c, ref.Speech.Performance)
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range ref.Cases {
		t.Run(scenario.Name, func(t *testing.T) {
			e := NewEngine(c)
			maxError := 0.0
			for index, row := range scenario.Rows {
				state := SpeechState{Active: row.Speech.Active, Waiting: row.Speech.Waiting, Position: row.Speech.Position, Session: row.Speech.Session}
				if row.Speech.HasPlan {
					state.Speech = speech
				}
				got := e.Sample(row.MS, state, row.Settings)
				if got.Mouth.From != row.Expected.Mouth.From || got.Mouth.To != row.Expected.Mouth.To {
					t.Fatalf("frame %d mouth %s/%s; expected %s/%s", index, got.Mouth.From, got.Mouth.To, row.Expected.Mouth.From, row.Expected.Mouth.To)
				}
				values, want := frameValues(got), frameValues(row.Expected)
				for component, value := range values {
					error := math.Abs(value - want[component])
					maxError = math.Max(maxError, error)
					if error > 1e-8 || math.IsNaN(value) {
						t.Fatalf("frame %d at %.3f ms component %d: %.12g expected %.12g (difference %.3g)", index, row.MS, component, value, want[component], error)
					}
				}
			}
			t.Logf("%d original-TS frames; maximum numerical error %.3g", len(scenario.Rows), maxError)
		})
	}
}
func assertJSONNumbers(t *testing.T, got, want any) {
	t.Helper()
	a, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var x, y any
	if err := json.Unmarshal(a, &x); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &y); err != nil {
		t.Fatal(err)
	}
	var compare func(string, any, any)
	compare = func(path string, x, y any) {
		t.Helper()
		switch a := x.(type) {
		case float64:
			b, ok := y.(float64)
			if !ok || math.Abs(a-b) > 1e-10 {
				t.Fatalf("%s = %v expected %v", path, x, y)
			}
		case map[string]any:
			b, ok := y.(map[string]any)
			if !ok || len(a) != len(b) {
				t.Fatalf("%s fields differ: %v vs %v", path, x, y)
			}
			for k, v := range a {
				compare(path+"."+k, v, b[k])
			}
		case []any:
			b, ok := y.([]any)
			if !ok || len(a) != len(b) {
				t.Fatalf("%s length %d expected %v", path, len(a), y)
			}
			for i, v := range a {
				compare(fmt.Sprintf("%s[%d]", path, i), v, b[i])
			}
		default:
			if x != y {
				t.Fatalf("%s = %v expected %v", path, x, y)
			}
		}
	}
	compare("reference", x, y)
}
