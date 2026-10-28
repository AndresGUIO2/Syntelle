package nativeanimation

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"syntelle/render-contract"
	"testing"
)

func articulationConfig() Config {
	return Config{Directed: true, Mandible: &rendercontract.MandibleRig{PoseJaw: map[string]float64{"Rest": 0, "A": 1, "EI": .7, "FV": .2, "MBP": 0}}}
}
func TestArticulationRetainsRepeatedEventsAndStableVowel(t *testing.T) {
	rate := 8000
	samples := make([]float32, rate)
	for i := range samples {
		samples[i] = float32(.3 * math.Sin(2*math.Pi*160*float64(i)/float64(rate)))
	}
	events := []NativeViseme{{6, 0}, {14, 200}, {6, 400}, {6, 600}, {0, 950}}
	p, err := PrepareSpeech(t.Context(), samples, rate, events, 1, articulationConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(p.articulation.native) != len(events) {
		t.Fatal("native context discarded")
	}
	s := BindSpeech(p, nil)
	for time := .25; time < .7; time += .02 {
		a := s.ArticulationAt(time)
		if math.Abs(a.JawOpen-.7) > .002 || math.Abs(a.LipOpening) > .002 {
			t.Fatal("stable vowel was pumped", time, a)
		}
	}
}
func TestLaughterJawFollowsBurstsAndReleasesWithoutFaceReset(t *testing.T) {
	rate := 8000
	samples := make([]float32, rate)
	for i := range samples {
		time := float64(i) / float64(rate)
		if time >= .1 && time < .22 || time >= .45 && time < .58 {
			samples[i] = float32(.3 * math.Sin(2*math.Pi*180*time))
		}
	}
	p, err := PrepareSpeech(t.Context(), samples, rate, []NativeViseme{{0, 0}, {0, 1000}}, 1, articulationConfig(), true)
	if err != nil {
		t.Fatal(err)
	}
	s := BindSpeech(p, nil)
	for _, span := range [][2]float64{{.1, .22}, {.45, .58}} {
		peakAt, peak := 0., 0.
		for time := span[0] - .04; time < span[1]+.08; time += .005 {
			a := s.ArticulationAt(time)
			if a.JawOpen > peak {
				peak, peakAt = a.JawOpen, time
			}
		}
		if peak < .6 || peakAt > span[1]+1./30 {
			t.Fatal("pulse peak detached", span, peakAt, peak)
		}
	}
	if s.ArticulationAt(.38).JawOpen > .2 {
		t.Fatal("jaw stayed open across gap")
	}
	if s.ArticulationAt(.35).LaughFace <= 0 {
		t.Fatal("face reset between chuckles")
	}
}
func TestCachedArticulationEvidence(t *testing.T) {
	root := os.Getenv("SYNTELLE_ARTICULATION_REVIEW")
	if root == "" {
		t.Skip("explicit cached review only")
	}
	rigBytes, err := os.ReadFile(filepath.Join(root, "prepared-current", "mandibular-rig.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rig rendercontract.MandibleRig
	json.Unmarshal(rigBytes, &rig)
	c := articulationConfig()
	c.Mandible = &rig
	cases := []map[string]any{}
	for _, language := range []string{"en-US", "es-CO"} {
		data, err := os.ReadFile(filepath.Join(root, "inputs", language+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var r struct {
			Audio    string         `json:"audio_base64"`
			Visemes  []NativeViseme `json:"visemes"`
			Duration float64        `json:"duration_ms"`
		}
		if json.Unmarshal(data, &r) != nil {
			t.Fatal("invalid record")
		}
		wav, _ := base64.StdEncoding.DecodeString(r.Audio)
		var pcm []byte
		rate := 24000
		for at := 12; at+8 <= len(wav); {
			size := int(binary.LittleEndian.Uint32(wav[at+4:]))
			if at+8+size > len(wav) {
				t.Fatal("truncated WAV")
			}
			if string(wav[at:at+4]) == "data" {
				pcm = wav[at+8 : at+8+size]
				break
			}
			at += 8 + size + size%2
		}
		samples := make([]float32, len(pcm)/2)
		for i := range samples {
			samples[i] = float32(int16(binary.LittleEndian.Uint16(pcm[i*2:]))) / 32768
		}
		p, err := PrepareSpeech(context.Background(), samples, rate, r.Visemes, 1, c, true)
		if err != nil {
			t.Fatal(err)
		}
		speech := BindSpeech(p, nil)
		frames := []map[string]any{}
		c.Emotions, c.Expressions, c.SmileAvailable, c.SpeakingSmile, c.LaughAvailable, c.ReferenceLaugh, c.OralRecovery = true, true, true, true, true, true, true
		amount := 1.
		speech = BindSpeech(p, &Performance{Intent: &PerformanceIntent{Version: "performance-intent/2"}, Resolved: []PerformanceCue{{Kind: "laugh", Duration: r.Duration / 1000, Amount: &amount}}})
		engine := NewEngine(c)
		settings := DefaultSettings()
		settings.Movement = false
		settings.Blinking = false
		settings.EmotionBaseline = rendercontract.EmotionBaseline{State: "playful", Intensity: .5}
		for i := 0; i < int(math.Ceil((r.Duration/1000+.6)*30)); i++ {
			time := float64(i) / 30
			active := time < r.Duration/1000
			frame := engine.Sample(time*1000, SpeechState{Active: active, Session: 1, Position: time, Speech: speech}, settings)
			a := speech.ArticulationAt(time)
			state := map[string]any{"From": "Rest", "To": "A", "Amount": 0, "JawOpen": frame.JawOpen, "Laugh": frame.Laugh, "Emotion": frame.Emotion, "Background": "dark-lounge"}
			cases = append(cases, map[string]any{"name": language + "-" + fmtIndex(i), "state": state})
			frames = append(frames, map[string]any{"time": time, "channels": a})
		}
		result, _ := json.MarshalIndent(map[string]any{"version": ArticulationVersion, "last_burst": p.articulation.lastBurst, "frames": frames, "native_events": p.articulation.native, "frames_20ms": p.articulation.frames}, "", "  ")
		os.WriteFile(filepath.Join(root, language+"-envelopes.json"), result, 0600)
	}
	encoded, _ := json.Marshal(cases)
	os.WriteFile(filepath.Join(root, "laugh-states.json"), encoded, 0600)
}
func fmtIndex(i int) string {
	b := []byte{'0', '0', '0'}
	for j := 2; j >= 0; j-- {
		b[j] = byte('0' + i%10)
		i /= 10
	}
	return string(b)
}

func TestJawSurvivesHeadDisablePauseAndNewSpeech(t *testing.T) {
	c := articulationConfig()
	e := NewEngine(c)
	settings := DefaultSettings()
	settings.Movement = false
	settings.Blinking = false
	samples := make([]float32, 8000)
	for i := range samples {
		samples[i] = float32(.2 * math.Sin(float64(i)*.2))
	}
	p, err := PrepareSpeech(t.Context(), samples, 8000, []NativeViseme{{1, 0}, {1, 1000}}, 1, c)
	if err != nil {
		t.Fatal(err)
	}
	s := BindSpeech(p, nil)
	f := e.Sample(200, SpeechState{Active: true, Position: .2, Session: 1, Speech: s}, settings)
	if f.JawOpen <= .1 || f.Head != (Head{}) {
		t.Fatal("jaw depends on head", f)
	}
	settings.Paused = true
	if frozen := e.Sample(1200, SpeechState{Active: true, Position: .2, Session: 1, Speech: s}, settings); frozen != f {
		t.Fatal("pause advanced jaw")
	}
	settings.Paused = false
	e.Sample(1200, SpeechState{}, settings)
	next := e.Sample(1210, SpeechState{Active: true, Position: .2, Session: 2, Speech: s}, settings)
	if next.JawOpen != f.JawOpen {
		t.Fatal("recovery overrode new speech")
	}
}

func TestRepeatedNativeContextUsesOnlySupportedAcousticVariation(t *testing.T) {
	c := articulationConfig()
	samples := make([]float32, 8000)
	for i := range samples {
		time := float64(i) / 8000
		samples[i] = float32((.2 + .15*math.Sin(2*math.Pi*6*time)) * math.Sin(2*math.Pi*180*time))
	}
	events := []NativeViseme{{4, 0}, {4, 100}, {5, 200}, {6, 350}, {14, 450}, {21, 600}, {18, 720}, {4, 840}, {0, 1000}}
	p, err := PrepareSpeech(t.Context(), samples, 8000, events, 1, c)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.articulation.native) != len(events) {
		t.Fatal("repeated native events lost")
	}
	if p.timeline[0].Pose != "EI" || p.timeline[1].Time < .59 {
		t.Fatal("identical poses introduced artificial reset", p.timeline)
	}
	changed := false
	for i, f := range p.articulation.frames {
		m := MouthAt(p.timeline, float64(i)*.02)
		if mouthContact(m.From) || mouthContact(m.To) {
			if f.LipOpening != 0 || f.LipTension != 0 {
				t.Fatal("acoustics overrode contact")
			}
		}
		if math.Abs(f.LipOpening) > .05 || math.Abs(f.LipTension) > .05 {
			t.Fatal("residual exceeded calibrated cap")
		}
		changed = changed || math.Abs(f.LipOpening) > .0001
	}
	if !changed {
		t.Fatal("acoustic context lost within merged EI run")
	}
}

func TestMandibleBridgesShortPhraseWithoutRetimingContacts(t *testing.T) {
	c := articulationConfig()
	e := NewEngine(c)
	settings := DefaultSettings()
	settings.Movement = false
	samples := make([]float32, 8000)
	p, err := PrepareSpeech(t.Context(), samples, 8000, []NativeViseme{{1, 0}, {1, 1000}}, 1, c)
	if err != nil {
		t.Fatal(err)
	}
	first := BindSpeech(p, nil)
	before := e.Sample(200, SpeechState{Active: true, Session: 1, Position: .2, Speech: first}, settings)
	next, err := PrepareSpeech(t.Context(), samples, 8000, []NativeViseme{{21, 0}, {1, 100}, {0, 1000}}, 1, c)
	if err != nil {
		t.Fatal(err)
	}
	following := BindSpeech(next, nil)
	boundary := e.Sample(210, SpeechState{Active: true, Session: 2, Position: 0, Speech: following}, settings)
	if boundary.JawOpen != before.JawOpen {
		t.Fatal("gross jaw reset at phrase boundary")
	}
	if boundary.Mouth != MouthAt(following.Timeline, 0) {
		t.Fatal("jaw bridge retimed lip contact")
	}
	after := e.Sample(250, SpeechState{Active: true, Session: 2, Position: .04, Speech: following}, settings)
	if math.Abs(after.JawOpen-following.ArticulationAt(.04).JawOpen) > 1e-9 {
		t.Fatal("bridge delayed new jaw trajectory")
	}
}
