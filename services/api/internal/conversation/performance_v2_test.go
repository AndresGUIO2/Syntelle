package conversation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPhraseDirectionPreservesTextAndNumericExpressions(t *testing.T) {
	for _, test := range []struct {
		name, cues string
		want       int
	}{
		{"numeric", `[{"kind":"brows","anchor":"Hello","amount":0.5}]`, 1},
		{"invalid amount", `[{"kind":"brows","anchor":"Hello","amount":1.1}]`, 0},
		{"invalid optional array", `42`, 0},
		{"unknown optional field", `[{"kind":"brows","anchor":"Hello","amount":0.5,"execute":"x"}]`, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw := `{"version":"performance-intent/2","tone":"neutral","phrases":[{"text":"Hello there.","cues":` + test.cues + `}]}`
			p, err := completePhrases(raw)
			if err != nil || len(p) != 1 || p[0].Text != "Hello there." || len(p[0].Cues) != test.want {
				t.Fatal(p, err)
			}
			for end := 0; end < len(raw); end++ {
				prefix, err := decodedPhrases(raw[:end])
				if err != nil {
					t.Fatal(err)
				}
				if len(prefix) > 0 && !strings.Contains(raw[:end], test.cues+`}`) {
					t.Fatal("incomplete phrase admitted")
				}
			}
		})
	}
}

func TestPhraseGesturesShareOneReplyAllowance(t *testing.T) {
	b := newReplyGestures()
	amount := .5
	for i := range 12 {
		v := PerformanceValidation{Resolved: []ResolvedCue{{Kind: "nod", Time: .2, Duration: .85, Amount: &amount}}}
		got := b.admit(v, float64(i)*3)
		if (i < 8 && len(got.Resolved) != 1) || (i >= 8 && len(got.Resolved) != 0) {
			t.Fatal(i, got)
		}
	}
}

func TestAcousticOpportunitiesContinueAcrossShortPhrases(t *testing.T) {
	a := newAcousticSchedule("fixture")
	first := a.next
	v := PerformanceValidation{Intent: PerformanceIntent{Version: "performance-intent/2", Text: "A short phrase."}}
	energy := make([]float64, 100)
	for i := range energy {
		energy[i] = .1
	}
	// A two-second phrase straddles an opportunity in the reply's audio clock.
	got := a.supplement(v, []float64{.5}, energy, .1, 2, first-.5)
	if len(got.Resolved) != 1 || got.Resolved[0].Kind != "emphasis" || got.Resolved[0].Source != "acoustic" {
		t.Fatal(got)
	}
	if gap := a.next - first; gap < 4 || gap > 7 {
		t.Fatal("opportunity spacing", gap)
	}
}

func TestGeminiPhraseArrivesBeforeProviderCompletion(t *testing.T) {
	delivered := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"event_type\":\"step.start\",\"index\":0,\"step\":{\"type\":\"model_output\"}}\n\n"))
		fragment := func(value string) {
			e, _ := json.Marshal(map[string]any{"event_type": "step.delta", "index": 0, "delta": map[string]string{"type": "text", "text": value}})
			_, _ = w.Write(append(append([]byte("data: "), e...), []byte("\n\n")...))
			w.(http.Flusher).Flush()
		}
		fragment(`{"version":"performance-intent/2","tone":"neutral","phrases":[{"text":"Hello there.","cues":[{"kind":"brows","anchor":"Hello","amount":0.5}]}`)
		select {
		case <-delivered:
		case <-r.Context().Done():
			return
		}
		fragment(`,{"text":"Another complete thought.","cues":[]}]}`)
		_, _ = w.Write([]byte("data: {\"event_type\":\"interaction.completed\",\"interaction\":{\"status\":\"completed\",\"model\":\"gemini-3.8-flash\"}}\n\n"))
	}))
	defer server.Close()
	m, _ := NewGemini("fixture", GeminiChatModel)
	m.endpoint = server.URL
	m.WithDirection()
	count := 0
	result, err := m.CompleteWithPhrases(context.Background(), "", nil, func(p PerformancePhrase) error {
		count++
		if count == 1 {
			close(delivered)
		}
		return nil
	})
	if err != nil || count != 2 || result.Text != "Hello there. Another complete thought." {
		t.Fatal(result, err, count)
	}
}
