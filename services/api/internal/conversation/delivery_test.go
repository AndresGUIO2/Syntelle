package conversation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"syntelle/render-contract"
	"testing"
	"time"
)

type coordinatedModel struct{ count int }

func (coordinatedModel) Settings() any                       { return "offline-fixture" }
func (coordinatedModel) PhraseDirection() bool               { return true }
func (coordinatedModel) PhraseRequest(string, []Message) any { return "offline-fixture" }
func (coordinatedModel) Complete(context.Context, string, []Message, func(string)) (ModelResult, error) {
	panic("unexpected legacy model call")
}
func (m coordinatedModel) CompleteWithPhrases(ctx context.Context, _ string, _ []Message, emit func(PerformancePhrase) error) (ModelResult, error) {
	texts := []string{"We can take this one step at a time.", "What would you like to try first?"}
	if m.count > 0 {
		texts = nil
		for i := 0; i < m.count; i++ {
			texts = append(texts, fmt.Sprintf("Complete thought %d.", i))
		}
	}
	for _, text := range texts {
		if err := emit(PerformancePhrase{Version: rendercontract.CoordinatedPerformanceVersion, Text: text, Tone: "neutral", Purpose: "reassure", Cues: []PerformanceCue{}}); err != nil {
			return ModelResult{}, err
		}
	}
	return ModelResult{Text: strings.Join(texts, " ")}, ctx.Err()
}

func TestCoordinatedReplyContinuesBeyondThirtyTwoPhrases(t *testing.T) {
	store := isolatedStore(t)
	speech := &coordinatedSpeech{}
	c := &coordinatedCoordinator{}
	s := New(store, coordinatedModel{count: 34}, speech, nil)
	s.Delivery = c
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cid, id := "long_coordinated_session", "long_coordinated_turn"
	if _, err := store.Session(ctx, cid, "syntelle", "en-US", "Fixture"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginTurn(ctx, cid, id, "Full answer"); err != nil {
		t.Fatal(err)
	}
	parts, completed := 0, false
	s.runTurn(ctx, cid, id, "Full answer", "en-US", rendercontract.DefaultVoice, "Fixture", func(e Event) {
		if e.Type == "error" {
			t.Error(e.Text)
		}
		if e.Type == "speech_part" {
			parts++
			c.complete("activated")
		}
		if e.Type == "speech_complete" {
			completed = true
		}
	})
	if !completed || parts != 34 || speech.calls.Load() != 34 {
		t.Fatal("whole reply was capped", parts, speech.calls.Load())
	}
}

type coordinatedSpeech struct{ calls atomic.Int32 }

func (s *coordinatedSpeech) Prepare(context.Context, string, string, string, string) (any, error) {
	panic("unapproved synthesis")
}
func (s *coordinatedSpeech) VoiceDirection(r rendercontract.DeliveryReservation) (rendercontract.VoiceDelivery, error) {
	return rendercontract.VoiceDelivery{Version: rendercontract.DeliveryVersion, Purpose: r.Request.Purpose, Emotion: r.Effective, CapabilityRevision: "fixture/1"}, nil
}
func (s *coordinatedSpeech) PrepareDirected(ctx context.Context, text, language, voice, id string, d rendercontract.VoiceDelivery) (any, error) {
	s.calls.Add(1)
	return map[string]any{"audio_base64": performanceFixtureWAV(), "duration_ms": 9000, "voice": voice, "language": language}, ctx.Err()
}

type coordinatedCoordinator struct {
	mu       sync.Mutex
	ticket   *DeliveryTicket
	updates  chan rendercontract.DeliveryNotice
	revision uint64
	requests int
}

func (c *coordinatedCoordinator) ReserveDelivery(ctx context.Context, _ string, r rendercontract.DeliveryRequest) (*DeliveryTicket, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.revision == 0 {
		c.revision = 1
	}
	c.requests++
	r.Revision = c.revision
	r.RequestID = fmt.Sprintf("fixture_%d", c.requests)
	c.updates = make(chan rendercontract.DeliveryNotice, 1)
	c.ticket = &DeliveryTicket{Reservation: rendercontract.DeliveryReservation{Version: rendercontract.DeliveryVersion, Token: r.RequestID, Request: r, Instance: "fixture_instance", Effective: rendercontract.EmotionTarget{State: "neutral", Energy: .5}}, Updates: c.updates, Close: func() {}}
	return c.ticket, ctx.Err()
}
func (c *coordinatedCoordinator) complete(status string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	r := c.ticket.Reservation
	if status == "invalidated" {
		c.revision++
	}
	c.updates <- rendercontract.DeliveryNotice{RequestID: r.Request.RequestID, Token: r.Token, Status: status, Reason: "superseded_by_settings", Reservation: &r}
}

func TestCoordinatedTurnWaitsForActualActivationAndReplacesOnlyUnplayedText(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(fmt.Sprint(replace), func(t *testing.T) {
			store := isolatedStore(t)
			speech := &coordinatedSpeech{}
			coordinator := &coordinatedCoordinator{}
			s := New(store, coordinatedModel{}, speech, nil)
			s.Delivery = coordinator
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cid, id := "coordinated_session", "coordinated_turn"
			if _, err := store.Session(ctx, cid, "syntelle", "en-US", "Fixture"); err != nil {
				t.Fatal(err)
			}
			if _, err := store.BeginTurn(ctx, cid, id, "Hello"); err != nil {
				t.Fatal(err)
			}
			parts := 0
			final := ""
			firstText := ""
			s.runTurn(ctx, cid, id, "Hello", "en-US", rendercontract.DefaultVoice, "Fixture", func(e Event) {
				if e.Type == "error" {
					t.Error(e.Text)
				}
				if e.Type == "speech_part" {
					if parts == 0 {
						played, err := store.PlaybackText(ctx, cid, id)
						if err != nil || played != "" {
							t.Error("unplayed text entered echo context", played, err)
						}
					}
					parts++
					b, _ := json.Marshal(e.Data)
					var p SpeechPart
					if err := json.Unmarshal(b, &p); err != nil {
						t.Error(err)
					}
					if parts == 1 {
						firstText = p.Text
						if speech.calls.Load() != 1 {
							t.Error("next phrase synthesized before activation")
						}
					}
					if replace && parts == 1 {
						coordinator.complete("invalidated")
					} else {
						if replace && parts == 2 && p.Text != firstText {
							t.Error("replacement changed spoken text")
						}
						coordinator.complete("activated")
					}
				}
				if e.Type == "reply_complete" {
					final = e.Text
				}
			})
			want := 2
			if replace {
				want = 3
			}
			if parts != want || int(speech.calls.Load()) != want || final != "We can take this one step at a time. What would you like to try first?" {
				t.Fatal(parts, speech.calls.Load(), final)
			}
			played, err := store.PlaybackText(ctx, cid, id)
			if err != nil || played != final {
				t.Fatal("played text omitted or repeated a replacement", played, err)
			}
		})
	}
}
