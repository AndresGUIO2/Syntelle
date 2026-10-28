package nativeadapter

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"math"
	"os"
	"testing"
	"time"

	"syntelle/render-contract"
	"syntelle/render-worker/internal/nativeanimation"
	"syntelle/render-worker/internal/nativemedia"
	"syntelle/render-worker/internal/nativerender"
)

func fixtureScene(t *testing.T) (*scene, nativemedia.SpeechState) {
	t.Helper()
	raw, err := os.ReadFile("../nativeanimation/testdata/animation-fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Bundle struct{ Motion, Gaze, Expressions, Oral json.RawMessage }
		Speech struct {
			Audio       string `json:"audio_base64"`
			Rate        int    `json:"sample_rate"`
			Text        string
			Visemes     []nativemedia.Viseme
			Performance json.RawMessage
		}
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	c, err := nativeanimation.LoadBundle(fixture.Bundle.Motion, fixture.Bundle.Gaze, fixture.Bundle.Expressions, fixture.Bundle.Oral)
	if err != nil {
		t.Fatal(err)
	}
	// This fixture records the original TypeScript profile, before the optional
	// expressive trajectory. Keep its historical animation contract explicit.
	c.Expressive = false
	wav, err := base64.StdEncoding.DecodeString(fixture.Speech.Audio)
	if err != nil {
		t.Fatal(err)
	}
	var pcm []int16
	for at := 12; at+8 <= len(wav); {
		size := int(binary.LittleEndian.Uint32(wav[at+4:]))
		end := at + 8 + size
		if end > len(wav) {
			t.Fatal("invalid WAV fixture")
		}
		if string(wav[at:at+4]) == "data" {
			pcm = make([]int16, size/2)
			for i := range pcm {
				pcm[i] = int16(binary.LittleEndian.Uint16(wav[at+8+i*2:]))
			}
		}
		at = end + size%2
	}
	scene := &scene{config: c, engine: nativeanimation.NewEngine(c)}
	clip := nativemedia.SpeechState{Active: true, Samples: pcm, SampleRate: fixture.Speech.Rate, Visemes: fixture.Speech.Visemes, Duration: time.Duration(len(pcm)) * time.Second / time.Duration(fixture.Speech.Rate), Text: fixture.Speech.Text, Performance: fixture.Speech.Performance}
	clip.Prepared, err = scene.PrepareSpeech(t.Context(), clip)
	if err != nil {
		t.Fatal(err)
	}
	return scene, clip
}
func TestSceneAnimationClockFreezesAcrossPause(t *testing.T) {
	t.Parallel()
	s, _ := fixtureScene(t)
	input := nativemedia.FrameInput{Settings: rendercontract.Defaults()}
	if _, err := s.sample(input); err != nil {
		t.Fatal(err)
	}
	input.MediaTime = time.Second
	before, err := s.sample(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, ms := range []int{1100, 2000} {
		input.MediaTime = time.Duration(ms) * time.Millisecond
		input.Paused = true
		got, err := s.sample(input)
		if err != nil {
			t.Fatal(err)
		}
		if got.Head != before.Head || got.Breath != before.Breath || got.Closure != before.Closure {
			t.Fatal("animation moved while paused")
		}
	}
	input.MediaTime = 2100 * time.Millisecond
	input.Paused = false
	if _, err = s.sample(input); err != nil {
		t.Fatal(err)
	}
	input.MediaTime = 2200 * time.Millisecond
	if _, err = s.sample(input); err != nil {
		t.Fatal(err)
	}
	if s.animationTime != 1200*time.Millisecond || s.mediaTime != 2200*time.Millisecond {
		t.Fatalf("animation/media clocks %v/%v", s.animationTime, s.mediaTime)
	}
}

func TestWaitingRetainsOriginalRestingExpression(t *testing.T) {
	s, _ := fixtureScene(t)
	s.config.SpeakingSmile = true
	s.engine = nativeanimation.NewEngine(s.config)
	settings := rendercontract.Defaults()
	settings.Movement = false
	settings.Blinking = false
	settings.AutomaticExpressions = false
	settings.Smile = 1
	input := nativemedia.FrameInput{Settings: settings}
	_, _ = s.sample(input)
	input.MediaTime = time.Second
	idle, err := s.sample(input)
	if err != nil {
		t.Fatal(err)
	}
	input.Speech.Waiting = true
	for _, ms := range []int{1100, 1400, 2100} {
		input.MediaTime = time.Duration(ms) * time.Millisecond
		waiting, err := s.sample(input)
		if err != nil {
			t.Fatal(err)
		}
		if !waiting.RestingSmile || waiting.Smile != idle.Smile || waiting.From != idle.From || waiting.To != idle.To || waiting.Amount != idle.Amount {
			t.Fatalf("waiting changed the resting expression: idle=%+v waiting=%+v", idle, waiting)
		}
	}
}

func TestRepeatedNodPreviewFreezesAndInterruptionRestoresIdle(t *testing.T) {
	s, _ := fixtureScene(t)
	settings := rendercontract.Defaults()
	settings.Movement = false
	settings.Blinking = false
	input := nativemedia.FrameInput{Settings: settings, PreviewKind: "nod", PreviewAmount: 1, PreviewRepetitions: 7, PreviewSequence: 1}
	if _, err := s.sample(input); err != nil {
		t.Fatal(err)
	}
	input.MediaTime = 260 * time.Millisecond
	before, err := s.sample(input)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(before.Head[1]) < .5 {
		t.Fatal("nod did not move", before.Head)
	}
	input.Paused = true
	input.MediaTime = 3 * time.Second
	paused, err := s.sample(input)
	if err != nil || paused.Head != before.Head {
		t.Fatal("nod advanced while paused", paused.Head, err)
	}
	input.Paused = false
	input.PreviewKind = ""
	input.MediaTime = 3100 * time.Millisecond
	interrupted, err := s.sample(input)
	if err != nil || interrupted.Head != ([3]float64{}) {
		t.Fatal("nod retained after interruption", interrupted.Head, err)
	}
}
func TestSceneCachesSpeechRetainsCompletionAndClearsInterruption(t *testing.T) {
	t.Parallel()
	s, clip := fixtureScene(t)
	input := nativemedia.FrameInput{Generation: 1, Settings: rendercontract.Defaults(), Speech: clip}
	if _, err := s.sample(input); err != nil {
		t.Fatal(err)
	}
	prepared := s.speech
	session := s.animationSession
	if prepared == nil {
		t.Fatal("speech not prepared")
	}
	input.MediaTime = time.Second
	input.Speech.Position = time.Second
	input.Speech.Revision++
	if _, err := s.sample(input); err != nil {
		t.Fatal(err)
	}
	if s.speech != prepared || s.animationSession != session {
		t.Fatal("same immutable PCM was prepared again")
	}
	input.MediaTime = 2 * time.Second
	input.Speech = nativemedia.SpeechState{}
	completed, err := s.sample(input)
	if err != nil {
		t.Fatal(err)
	}
	if s.speech != prepared || completed.From != "Rest" || completed.To != "A" || completed.Amount != 0 {
		t.Fatal("completed phrase did not retain release plan with closed mouth")
	}
	input.MediaTime = 3 * time.Second
	input.Generation++
	if _, err := s.sample(input); err != nil {
		t.Fatal(err)
	}
	if s.speech != nil || s.clipPrepared != nil || s.animationSession <= session {
		t.Fatal("interruption retained old speech generation")
	}
}

func TestScenePreparationDefersPerformanceAdmissionUntilActivation(t *testing.T) {
	t.Parallel()
	s, first := fixtureScene(t)
	second := first
	var err error
	second.Prepared, err = s.PrepareSpeech(t.Context(), second)
	if err != nil {
		t.Fatal(err)
	}
	smileCount := func(value *nativeanimation.Performance) int {
		count := 0
		if value != nil {
			for _, cue := range value.Resolved {
				if cue.Kind == "smile" {
					count++
				}
			}
		}
		return count
	}
	input := nativemedia.FrameInput{MediaTime: 100 * time.Second, Settings: rendercontract.Defaults(), Speech: first}
	if _, err = s.sample(input); err != nil {
		t.Fatal(err)
	}
	if smileCount(s.speech.Plan.Performance) != 1 {
		t.Fatal("preparing future speech spent the first activation's smile allowance")
	}
	input.MediaTime += 100 * time.Millisecond
	input.Speech = second
	if _, err = s.sample(input); err != nil {
		t.Fatal(err)
	}
	if smileCount(s.speech.Plan.Performance) != 0 || smileCount(first.Prepared.Performance) != 1 || smileCount(second.Prepared.Performance) != 1 {
		t.Fatal("activation changed admission cooldown semantics or mutated prepared intent")
	}
}
func TestSceneWaitingSettlesManualSmile(t *testing.T) {
	t.Parallel()
	s, _ := fixtureScene(t)
	settings := rendercontract.Defaults()
	settings.Smile = .8
	input := nativemedia.FrameInput{Settings: settings}
	if _, err := s.sample(input); err != nil {
		t.Fatal(err)
	}
	input.MediaTime = time.Second
	smiling, err := s.sample(input)
	if err != nil {
		t.Fatal(err)
	}
	if smiling.Smile < .79 {
		t.Fatal("manual smile did not settle")
	}
	input.MediaTime = 1100 * time.Millisecond
	input.Speech.Waiting = true
	waiting, err := s.sample(input)
	if err != nil {
		t.Fatal(err)
	}
	if waiting.Smile != 0 {
		t.Fatal("waiting speech retained open smile material")
	}
}

// recordedRenderer observes the exact state handed across the scene boundary;
// pixel equivalence is covered separately by the native rendering proof.
type recordedRenderer struct{ frame nativerender.FrameState }

func (r *recordedRenderer) Render(_ context.Context, state nativerender.FrameState, _ *image.RGBA) error {
	r.frame = state
	return nil
}
func TestSceneReplaysOriginalTypeScriptRecordedFrames(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("../nativeanimation/testdata/animation-fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Name string
			Rows []struct {
				MS       float64
				MediaNS  time.Duration `json:"media_ns"`
				Settings nativeanimation.Settings
				Speech   struct {
					Active, Waiting bool
					Position        float64
					HasPlan         bool `json:"has_plan"`
				}
				Expected nativeanimation.Frame
			}
		}
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range fixture.Cases {
		t.Run(scenario.Name, func(t *testing.T) {
			s, clip := fixtureScene(t)
			recorder := &recordedRenderer{}
			s.renderer = recorder
			maximum := 0.0
			for index, row := range scenario.Rows {
				settings := rendercontract.Defaults()
				settings.Movement = row.Settings.Movement
				settings.Blinking = row.Settings.Blinking
				settings.AutomaticExpressions = row.Settings.AutomaticExpressions
				settings.Smile = row.Settings.Smile
				settings.Brows = row.Settings.Brows
				settings.ReducedMotion = row.Settings.ReducedMotion
				input := nativemedia.FrameInput{MediaTime: row.MediaNS, Settings: settings, Paused: row.Settings.Paused}
				input.Speech.Active = row.Speech.Active
				input.Speech.Waiting = row.Speech.Waiting
				input.Speech.Position = time.Duration(row.Speech.Position * float64(time.Second))
				if row.Speech.Active {
					input.Speech = clip
					input.Speech.Active = true
					input.Speech.Position = time.Duration(row.Speech.Position * float64(time.Second))
				}
				if err = s.Render(context.Background(), input, nil); err != nil {
					t.Fatal(err)
				}
				got := recorder.frame
				want := row.Expected
				if got.From != want.Mouth.From || got.To != want.Mouth.To {
					t.Fatalf("frame %d mouth %s/%s expected %s/%s", index, got.From, got.To, want.Mouth.From, want.Mouth.To)
				}
				values := []float64{got.Head[0], got.Head[1], got.Head[2], got.Breath, got.Amount, got.Closure, got.Gaze[0][0], got.Gaze[0][1], got.Gaze[1][0], got.Gaze[1][1], got.Smile, got.Brows}
				expected := []float64{want.Head.Yaw, want.Head.Pitch, want.Head.Roll, want.Breath, want.Mouth.Amount, want.EyeClosure, want.Gaze[0][0], want.Gaze[0][1], want.Gaze[1][0], want.Gaze[1][1], want.Smile, want.Brows}
				for component, value := range values {
					difference := math.Abs(value - expected[component])
					maximum = math.Max(maximum, difference)
					if difference > 1e-7 || math.IsNaN(value) {
						t.Fatalf("frame %d at %.3f ms component %d = %.12g expected %.12g (difference %.3g)", index, row.MS, component, value, expected[component], difference)
					}
				}
			}
			t.Logf("%d recorded TS frames crossed media/scene boundary; max error %.3g", len(scenario.Rows), maximum)
		})
	}
}

func TestAppendedLaughterUsesSpanRelativePulseAnalysis(t *testing.T) {
	s, _ := fixtureScene(t)
	s.config.Mandible = &rendercontract.MandibleRig{PoseJaw: map[string]float64{"Rest": 0, "A": 1}, PoseGap: map[string]float64{"Rest": 0, "A": 50}}
	s.config.ReferenceLaugh = true
	s.config.LaughAvailable = true
	s.engine = nativeanimation.NewEngine(s.config)
	samples := make([]int16, 48000)
	for i := 24000; i < 48000; i++ {
		samples[i] = int16(12000 * math.Sin(float64(i)*2*math.Pi*180/24000))
	}
	pcm := make([]byte, 48000)
	for i, v := range samples[24000:] {
		binary.LittleEndian.PutUint16(pcm[i*2:], uint16(v))
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(pcm))
	amount := 1.
	p := nativeanimation.Performance{Intent: &nativeanimation.PerformanceIntent{Version: "performance-intent/2", Text: "cached mixed", Cues: []nativeanimation.PerformanceAnchor{{Kind: "laugh", Anchor: "mixed"}}}, Resolved: []nativeanimation.PerformanceCue{{Kind: "laugh", Time: 1, Duration: 1, Amount: &amount}}}
	raw, _ := json.Marshal(p)
	clip := nativemedia.SpeechState{Active: true, TurnID: "mixed", Samples: samples, SampleRate: 24000, Duration: 2 * time.Second, Text: "cached mixed", Performance: raw, Visemes: []nativemedia.Viseme{{ID: 1, OffsetMS: 0}, {ID: 0, OffsetMS: 1000}, {ID: 0, OffsetMS: 2000}}, LaughterSpan: &rendercontract.LaughterSpan{Version: "laughter-span/1", Offset: 24000, Count: 24000, PCMHash: hash, RecordingHash: hash}}
	var err error
	clip.Prepared, err = s.PrepareSpeech(t.Context(), clip)
	if err != nil {
		t.Fatal(err)
	}
	clip.Position = 1300 * time.Millisecond
	f, err := s.sample(nativemedia.FrameInput{MediaTime: clip.Position, Speech: clip, Settings: rendercontract.Defaults()})
	if err != nil {
		t.Fatal(err)
	}
	if f.Laugh < .7 || f.JawOpen < .5 {
		t.Fatal("full audible appended laugh lost its analysis", f.Laugh, f.JawOpen)
	}
}
