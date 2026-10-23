package conversation

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPerformanceOptionalCues(t *testing.T) {
	for _, test := range []struct {
		name, tone, text, kind, anchor string
		want                           int
	}{
		{"affirmative", "neutral", "That is correct.", "nod", "correct", 1},
		{"meaningful negative", "serious", "Please do not ignore the alarm.", "shake", "do not ignore", 1},
		{"neutral no is not inferred", "neutral", "No problem.", "", "", 0},
		{"playful", "playful", "That joke worked.", "laugh", "joke", 1},
		{"serious laughter rejected", "serious", "The alarm is real.", "laugh", "real", 0},
		{"truncated anchor", "neutral", "An answer.", "nod", "answer and more", 0},
		{"repeated anchor", "neutral", "yes, yes", "nod", "yes", 0},
		{"unknown cue", "neutral", "A reply.", "navigate", "reply", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			intent := PerformanceIntent{Version: "performance-intent/1", Text: test.text, Tone: test.tone, Cues: []PerformanceCue{{Kind: test.kind, Anchor: test.anchor}}}
			got := ValidatePerformance(intent, test.text)
			if len(got.Intent.Cues) != test.want {
				t.Fatalf("%+v", got)
			}
		})
	}
	intent, err := decodePerformance(`{"version":"performance-intent/1","text":"Hello.","tone":"neutral","cues":[{"kind":"nod","anchor":"Hello","url":"https://invalid"},42]}`)
	if err != nil || len(intent.Cues) != 0 || intent.Text != "Hello." {
		t.Fatal(intent, err)
	}
	for _, raw := range []string{`{"text":"[laughter]"}`, `{"text":"<speak>Hello.</speak>"}`, `{"text":"Hello.","execute":"x"}`, `{"text":"Hello."} {}`, `{"text":"incomplete`} {
		if _, err := decodePerformance(raw); err == nil {
			t.Fatal("accepted unsafe envelope", raw)
		}
	}
}

func TestStructuredStreamExposesOnlyDecodedText(t *testing.T) {
	raw := `{"version":"performance-intent/1","text":"Yes, \"that's right\". Hola 😀.","tone":"neutral","cues":[{"kind":"nod","anchor":"right"}]}`
	var emitted []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if json.NewDecoder(r.Body).Decode(&body) != nil || body["response_format"] == nil {
			t.Error("missing structured schema")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"event_type\":\"step.start\",\"index\":0,\"step\":{\"type\":\"model_output\"}}\n\n"))
		for _, fragment := range strings.SplitAfter(raw, ",") {
			event, _ := json.Marshal(map[string]any{"event_type": "step.delta", "index": 0, "delta": map[string]string{"type": "text", "text": fragment}})
			_, _ = w.Write(append(append([]byte("data: "), event...), []byte("\n\n")...))
		}
		_, _ = w.Write([]byte("data: {\"event_type\":\"interaction.completed\",\"interaction\":{\"status\":\"completed\",\"model\":\"gemini-3.8-flash\"}}\n\n"))
	}))
	defer server.Close()
	model, _ := NewGemini("fixture", GeminiChatModel)
	model.endpoint = server.URL
	model.WithDirection()
	result, err := model.Complete(context.Background(), "", nil, func(text string) { emitted = append(emitted, text) })
	if err != nil || strings.Join(emitted, "") != "Yes, \"that's right\". Hola 😀." || result.Performance == nil {
		t.Fatal(result, emitted, err)
	}
}

func TestResolvePerformanceHeadSpacingAndBounds(t *testing.T) {
	const seconds = 9
	wav := performanceFixtureWAV()
	intent := PerformanceIntent{Version: "performance-intent/1", Text: "Yes indeed, a longer useful answer follows. No, that part is incorrect. A final thought.", Tone: "neutral", Cues: []PerformanceCue{{Kind: "nod", Anchor: "Yes indeed"}, {Kind: "shake", Anchor: "incorrect"}, {Kind: "emphasis", Anchor: "final"}}}
	got := ResolvePerformance(ValidatePerformance(intent, intent.Text), wav, seconds)
	last := -3.
	if len(got.Resolved) > 2 {
		t.Fatal(got)
	}
	for _, cue := range got.Resolved {
		if cue.Time-last < 2.4 || cue.Time+cue.Duration > seconds {
			t.Fatal(got)
		}
		last = cue.Time
	}
}

func performanceFixtureWAV() []byte {
	const seconds = 9
	wav := make([]byte, 44+24000*2*seconds)
	copy(wav, "RIFF")
	copy(wav[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(wav[16:], 16)
	binary.LittleEndian.PutUint16(wav[20:], 1)
	binary.LittleEndian.PutUint16(wav[22:], 1)
	binary.LittleEndian.PutUint32(wav[24:], 24000)
	binary.LittleEndian.PutUint16(wav[34:], 16)
	copy(wav[36:], "data")
	binary.LittleEndian.PutUint32(wav[40:], uint32(len(wav)-44))
	for i := 44; i < len(wav); i += 2 {
		if (i/2)%24000 < 18000 {
			binary.LittleEndian.PutUint16(wav[i:], 6000)
		}
	}
	return wav
}

type directedModelFixture struct{}

func (directedModelFixture) Settings() any { return map[string]bool{"fixture": true} }
func (directedModelFixture) Complete(_ context.Context, _ string, _ []Message, delta func(string)) (ModelResult, error) {
	intent := PerformanceIntent{Version: "performance-intent/1", Text: "Yes, that is correct. A friendly example.", Tone: "playful", Cues: []PerformanceCue{{Kind: "nod", Anchor: "correct"}, {Kind: "smile", Anchor: "friendly"}, {Kind: "laugh", Anchor: "example"}}}
	validation := ValidatePerformance(intent, intent.Text)
	delta(intent.Text)
	return ModelResult{Text: intent.Text, Performance: &validation, Response: json.RawMessage(`{"immutable":"provider fixture"}`)}, nil
}

type directedSpeechFixture struct{}

func (directedSpeechFixture) Prepare(context.Context, string, string, string, string) (any, error) {
	return map[string]any{"audio_base64": performanceFixtureWAV(), "duration_ms": 9000}, nil
}
func TestDirectedTurnArchivesTimingWithoutChangingProviderEvidence(t *testing.T) {
	store := isolatedStore(t)
	service := New(store, directedModelFixture{}, directedSpeechFixture{}, nil)
	f := openSocketFixture(t, service, "performance-session-01")
	f.c.SetReadLimit(2 << 20)
	f.send(Event{Type: "submit", ConversationID: "performance-session-01", TurnID: "performance-turn-01", Text: "An affirmative example."})
	for {
		event := f.read()
		if event.Type == "error" {
			t.Fatal(event.Text)
		}
		if event.Type != "prepared_speech" {
			continue
		}
		data, err := json.Marshal(event.Data)
		if err != nil {
			t.Fatal(err)
		}
		var payload struct {
			Performance PerformanceValidation `json:"performance"`
			Audio       []byte                `json:"audio_base64"`
		}
		if err = json.Unmarshal(data, &payload); err != nil {
			t.Fatal(err)
		}
		if len(payload.Performance.Resolved) != 2 || string(payload.Audio) != string(performanceFixtureWAV()) {
			t.Fatal("lost direction or modified audio")
		}
		var raw, timing string
		if err = store.Pool.QueryRow(f.ctx, `SELECT result->'response'->>'immutable' FROM syntelle_conversation.operations WHERE turn_id=$1 AND kind='luna'`, event.TurnID).Scan(&raw); err != nil || raw != "provider fixture" {
			t.Fatal("modified provider evidence", err)
		}
		if err = store.Pool.QueryRow(f.ctx, `SELECT result->>'timing' FROM syntelle_conversation.operations WHERE turn_id=$1 AND kind='performance'`, event.TurnID).Scan(&timing); err != nil || timing != "audio-energy/1-approximate-text-spans" {
			t.Fatal("missing timing evidence", err)
		}
		return
	}
}
