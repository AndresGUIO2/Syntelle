//go:build integration

package conversation

import (
	"context"
	"strings"
	"testing"
	"time"
)

type identityPlainFixture struct{ input chan string }

func (m *identityPlainFixture) Settings() any {
	return map[string]bool{"offline_identity_fixture": true}
}
func (m *identityPlainFixture) Complete(_ context.Context, instructions string, _ []Message, delta func(string)) (ModelResult, error) {
	m.input <- instructions
	delta("Hello.")
	return ModelResult{Text: "Hello."}, nil
}

type identityPhraseFixture struct{ *identityPlainFixture }

func (*identityPhraseFixture) PhraseDirection() bool { return true }
func (*identityPhraseFixture) PhraseRequest(instructions string, messages []Message) any {
	return map[string]any{"instructions": instructions, "messages": messages}
}
func (m *identityPhraseFixture) CompleteWithPhrases(_ context.Context, instructions string, _ []Message, emit func(PerformancePhrase) error) (ModelResult, error) {
	m.input <- instructions
	err := emit(PerformancePhrase{Text: "Hello.", Tone: "neutral", Cues: []PerformanceCue{}})
	return ModelResult{Text: "Hello."}, err
}

func TestConfiguredIdentityThroughSession(t *testing.T) {
	for _, route := range []string{"plain", "phrases"} {
		for _, character := range []string{"syntelle", "cafe"} {
			t.Run(route+"-"+character, func(t *testing.T) {
				store := isolatedStore(t)
				capture := &identityPlainFixture{input: make(chan string, 2)}
				var model Model = capture
				if route == "phrases" {
					model = &identityPhraseFixture{capture}
				}
				s := New(store, model, &phraseTurnSpeech{}, nil)
				s.Performance = &PerformanceProfile{Identity: &CharacterIdentity{Version: "character-identity/1", CharacterID: "syntelle", Name: "Fixture identity", Presentation: "Feminine digital character"}}
				ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
				defer cancel()
				defer func() {
					if err := s.Shutdown(ctx); err != nil {
						t.Error(err)
					}
				}()
				cid := "identity-session-" + route + "-" + character
				for connection := range 2 {
					events := make(chan Event, 32)
					channel, err := s.OpenChannel(ctx, func(e Event) { events <- e })
					if err != nil {
						t.Fatal(err)
					}
					defer channel.Close()
					send := func(e Event) {
						t.Helper()
						e.ConversationID = cid
						if err := channel.Send(ctx, e); err != nil {
							t.Fatal(err)
						}
					}
					wait := func(kind string) {
						t.Helper()
						for {
							select {
							case e := <-events:
								if e.Type == "error" {
									t.Fatal(e.Text)
								}
								if e.Type == kind {
									return
								}
							case <-ctx.Done():
								t.Fatal(ctx.Err())
							}
						}
					}
					send(Event{Type: "hello", TurnID: "identity-hello-01", Character: character, Language: "es-CO", Voice: s.RuntimeVoice(), Instructions: "Original tone"})
					wait("ready")
					send(Event{Type: "configure", TurnID: "identity-configure-01", Instructions: "Owner tone: keep answers concise.", Voice: s.RuntimeVoice()})
					id := "identity-turn-first"
					if connection == 1 {
						id = "identity-turn-reconnected"
					}
					send(Event{Type: "submit", TurnID: id, Text: "Are you a real person?"})
					var instructions string
					select {
					case instructions = <-capture.input:
					case <-ctx.Done():
						t.Fatal(ctx.Err())
					}
					if strings.Contains(instructions, `"name":"Fixture identity"`) != (character == "syntelle") {
						t.Fatal("selected character identity missing or leaked", instructions)
					}
					for _, required := range []string{"Owner tone: keep answers concise.", "Selected language: es-CO", "Application-owned performance context"} {
						if !strings.Contains(instructions, required) {
							t.Fatal("context lost", required)
						}
					}
					if route == "plain" {
						wait("prepared_speech")
					} else {
						wait("speech_complete")
					}
					var archived string
					if err := store.Pool.QueryRow(ctx, `SELECT request#>>'{body,instructions}' FROM syntelle_conversation.operations WHERE turn_id=$1 AND kind IN ('luna','director')`, id).Scan(&archived); err != nil {
						t.Fatal(err)
					}
					if strings.Contains(archived, `"name":"Fixture identity"`) != (character == "syntelle") {
						t.Fatal("durable model request lost identity boundary")
					}
					channel.Close()
					select {
					case <-channel.Done():
					case <-ctx.Done():
						t.Fatal(ctx.Err())
					}
				}
			})
		}
	}
}
