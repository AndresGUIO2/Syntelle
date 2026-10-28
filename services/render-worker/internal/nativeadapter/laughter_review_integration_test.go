//go:build integration

package nativeadapter

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"syntelle/render-contract"
	"syntelle/render-worker/internal/nativeanimation"
	"syntelle/render-worker/internal/nativemedia"
	"syntelle/render-worker/internal/nativerender"
	"testing"
	"time"
)

func TestCompleteLaughterProductionReview(t *testing.T) {
	folder := os.Getenv("SYNTELLE_LAUGHTER_REVIEW")
	if folder == "" {
		t.Skip("explicit approved cached inputs")
	}
	assets, err := nativerender.Load(filepath.Join(folder, "prepared-joint/package"))
	if err != nil {
		t.Fatal(err)
	}
	config, err := nativeanimation.LoadBundle(assets.MotionJSON, assets.GazeJSON, assets.ExpressionJSON, assets.OralJSON)
	if err != nil {
		t.Fatal(err)
	}
	if !config.LaughterV2 {
		t.Fatal("review accidentally selected old resources")
	}
	data, err := os.ReadFile(filepath.Join(folder, "production-inputs.json"))
	if err != nil {
		t.Fatal(err)
	}
	var inputs []struct {
		Name, Text string
		Manual     bool
		Speech     struct {
			Audio       []byte                       `json:"audio_base64"`
			Visemes     []nativemedia.Viseme         `json:"visemes"`
			Performance json.RawMessage              `json:"performance"`
			Span        *rendercontract.LaughterSpan `json:"laughter_span"`
		}
	}
	if err = json.Unmarshal(data, &inputs); err != nil {
		t.Fatal(err)
	}
	var frames []map[string]any
	var decisions []map[string]any
	var envelopes []map[string]any
	manualChannels := map[string][]nativeanimation.ArticulationFrame{}
	for _, record := range inputs {
		t.Run(record.Name, func(t *testing.T) {
			wav := record.Speech.Audio
			var pcm []int16
			for at := 12; at+8 <= len(wav); {
				size := int(binary.LittleEndian.Uint32(wav[at+4:]))
				if at+8+size > len(wav) {
					t.Fatal("truncated original WAV")
				}
				if string(wav[at:at+4]) == "data" {
					pcm = make([]int16, size/2)
					for i := range pcm {
						pcm[i] = int16(binary.LittleEndian.Uint16(wav[at+8+i*2:]))
					}
					break
				}
				at += 8 + size + size%2
			}
			if err = nativeanimation.ValidateLaughterSpan(record.Speech.Span, pcm, 24000); err != nil {
				t.Fatal(err)
			}
			clip := nativemedia.SpeechState{Preview: record.Manual, Active: true, TurnID: record.Name, Samples: pcm, SampleRate: 24000, Duration: time.Duration(len(pcm)) * time.Second / 24000, Text: record.Text, Visemes: record.Speech.Visemes, Performance: record.Speech.Performance, LaughterSpan: record.Speech.Span}
			baseScene := &scene{config: config, engine: nativeanimation.NewEngine(config)}
			clip.Prepared, err = baseScene.PrepareSpeech(t.Context(), clip)
			if err != nil {
				t.Fatal(err)
			}
			admitted := baseScene.engine.AdmitPerformance(clip.Prepared.Performance, clip.Text, 0)
			speech := nativeanimation.BindSpeech(clip.Prepared, admitted)
			offset := float64(clip.LaughterSpan.Offset) / 24000
			duration := float64(clip.LaughterSpan.Count) / 24000
			channels := []nativeanimation.ArticulationFrame{}
			for position := 0.; position < duration; position += .02 {
				f, ok := speech.LaughterAt(offset + position)
				if !ok {
					t.Fatal("laughter span lost")
				}
				channels = append(channels, f)
			}
			envelopes = append(envelopes, map[string]any{"name": record.Name, "span": record.Speech.Span, "step_seconds": .02, "last_burst_seconds": speech.LaughterLastBurst() - offset, "channels": channels})
			language := record.Name[:5]
			if record.Manual {
				manualChannels[language] = channels
			} else {
				expected := manualChannels[language]
				if len(expected) != len(channels) {
					t.Fatal("manual/automatic length mismatch")
				}
				for i, f := range channels {
					g := expected[i]
					if math.Abs(f.JawOpen-g.JawOpen) > 1e-8 || math.Abs(f.LaughFace-g.LaughFace) > 1e-8 {
						t.Fatal("prefix/intention changed complete laugh", i, f, g)
					}
				}
			}
			for _, baseline := range []struct {
				Name             string
				Intensity, Smile float64
			}{{"neutral", 0, 0}, {"playful-half", .5, 0}, {"playful-full", 1, 0}, {"smile-full", 0, 1}, {"playful-smile", 1, .5}} {
				s := &scene{config: config, engine: nativeanimation.NewEngine(config)}
				settings := rendercontract.Defaults()
				settings.Movement = false
				settings.Blinking = false
				settings.Smile = baseline.Smile
				settings.EmotionBaseline = rendercontract.EmotionBaseline{State: "playful", Intensity: baseline.Intensity}
				if baseline.Intensity == 0 {
					settings.EmotionBaseline.State = "neutral"
				}
				if record.Manual && baseline.Name == "smile-full" {
					settings.AutomaticExpressions = false
				}
				var controller nativeanimation.EmotionController
				controller.Configure(0, settings, "")
				snapshot := controller.Motion
				input := nativemedia.FrameInput{Settings: settings, EmotionControl: &snapshot}
				s.sample(input)
				input.MediaTime = time.Second
				input.EmotionTime = 1000
				s.sample(input)
				for position := 0.; position < offset-.001; position += .05 {
					clip.Position = time.Duration(position * float64(time.Second))
					input.MediaTime = time.Second + clip.Position
					input.EmotionTime = float64(input.MediaTime / time.Millisecond)
					input.Speech = clip
					f, err := s.sample(input)
					if err != nil {
						t.Fatal(err)
					}
					if f.Laugh != 0 {
						t.Fatal("laugh before recorded span")
					}
				}
				peak := 0.
				var settled nativerender.FrameState
				count := 0
				for i := 0; i < int(math.Ceil((duration+.65)*30)); i++ {
					position := offset + float64(i)/30
					input.MediaTime = time.Second + time.Duration(position*float64(time.Second))
					input.EmotionTime = float64(input.MediaTime / time.Millisecond)
					clip.Position = time.Duration(position * float64(time.Second))
					input.Speech = clip
					if position >= offset+duration {
						input.Speech = nativemedia.SpeechState{}
					}
					frame, err := s.sample(input)
					if err != nil {
						t.Fatal(err)
					}
					peak = math.Max(peak, frame.Laugh)
					settled = frame
					// Use both routes and all fixed bank amounts for assertions; only the
					// representative .2 automatic action needs duplicate visual frames.
					if record.Manual || record.Name == language+"-automatic-2" {
						name := record.Name + "-" + baseline.Name + "-" + fmtIndex(i)
						frames = append(frames, map[string]any{"name": name, "state": frame})
						count++
					}
				}
				if peak < .999 || settled.Laugh != 0 {
					t.Fatal("partial/unfinished carcajada", peak, settled.Laugh)
				}
				expected, _ := snapshot.Sample(input.EmotionTime)
				if settled.Emotion != expected {
					t.Fatal("selected worker expression was not restored", settled.Emotion, expected)
				}
				decisions = append(decisions, map[string]any{"name": record.Name, "baseline": baseline.Name, "full_peak": peak, "restored": true, "frames": count, "automatic_expressions": settings.AutomaticExpressions})
			}
		})
	}
	encoded, _ := json.Marshal(frames)
	if err = os.WriteFile(filepath.Join(folder, "production-states.json"), encoded, 0644); err != nil {
		t.Fatal(err)
	}
	encoded, _ = json.MarshalIndent(envelopes, "", "  ")
	os.WriteFile(filepath.Join(folder, "production-envelopes.json"), encoded, 0644)
	encoded, _ = json.MarshalIndent(decisions, "", "  ")
	os.WriteFile(filepath.Join(folder, "production-decisions.json"), encoded, 0644)
}
func fmtIndex(n int) string {
	b := []byte{'0', '0', '0'}
	for i := 2; i >= 0; i-- {
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b)
}
