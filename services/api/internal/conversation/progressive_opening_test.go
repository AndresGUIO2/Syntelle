package conversation

import (
	"context"
	"errors"
	"syntelle/render-contract"
	"testing"
	"time"
)

type prefixSpeechFixture struct {
	coordinatedSpeech
	finish chan struct{}
	failed bool
}

func (s *prefixSpeechFixture) PrepareDirectedProgressive(ctx context.Context, text, language, voice, id string, d rendercontract.VoiceDelivery, publish func(any) error) (any, error) {
	value, err := s.PrepareDirected(ctx, text, language, voice, id, d)
	if err != nil {
		return nil, err
	}
	if err = publish(value); err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.finish:
	}
	if s.failed {
		return nil, errors.New("fixture failed after prefix")
	}
	return value, nil
}

func TestProgressiveOpeningDurableBeforeSynthesisCompletes(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "provider failure"}[failed], func(t *testing.T) {
			store := isolatedStore(t)
			voice := &prefixSpeechFixture{finish: make(chan struct{}), failed: failed}
			coordinator := &coordinatedCoordinator{}
			service := New(store, coordinatedModel{count: 1}, voice, nil)
			service.Delivery = coordinator
			ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
			defer cancel()
			cid, id := "progressive_opening_session", "progressive_opening_turn"
			if _, err := store.Session(ctx, cid, "syntelle", "en-US", "offline fixture"); err != nil {
				t.Fatal(err)
			}
			if _, err := store.BeginTurn(ctx, cid, id, "question"); err != nil {
				t.Fatal(err)
			}
			events := make(chan Event, 32)
			done := make(chan struct{})
			go func() {
				defer close(done)
				service.runTurn(ctx, cid, id, "question", "en-US", rendercontract.DefaultVoice, "offline fixture", func(e Event) { events <- e })
			}()
			t.Cleanup(func() { cancel(); <-done })
			var prefixes, finals, parts int
			completed, sawError := false, false
			for !completed && !sawError {
				select {
				case <-ctx.Done():
					t.Fatal("progressive turn stalled")
				case e := <-events:
					switch e.Type {
					case "speech_prefix":
						part := e.Data.(SpeechPart)
						if part.Final {
							finals++
							break
						}
						prefixes++
						var durable int
						if err := store.Pool.QueryRow(ctx, "SELECT count(*) FROM syntelle_conversation.operations WHERE kind='speech-prefix' AND status='received' AND audio IS NOT NULL").Scan(&durable); err != nil || durable != 1 {
							t.Fatal("prefix preceded durable bytes", err, durable)
						}
						select {
						case <-done:
							t.Fatal("opening waited for synthesis completion")
						default:
						}
						coordinator.complete("activated")
						// Echo protection must be ready before allowing the native call to finish.
						deadline := time.Now().Add(time.Second)
						for {
							text, err := store.PlaybackText(ctx, cid, id)
							if err != nil {
								t.Fatal(err)
							}
							if text != "" {
								break
							}
							if time.Now().After(deadline) {
								t.Fatal("early activation was missing from echo history")
							}
							time.Sleep(5 * time.Millisecond)
						}
						close(voice.finish)
					case "speech_part":
						parts++
					case "speech_complete":
						completed = true
					case "error":
						sawError = true
					}
				}
			}
			<-done
			if prefixes != 1 || parts != 0 || sawError != failed || completed == failed || finals != map[bool]int{true: 0, false: 1}[failed] {
				t.Fatal(prefixes, finals, parts, sawError, completed)
			}
		})
	}
}
