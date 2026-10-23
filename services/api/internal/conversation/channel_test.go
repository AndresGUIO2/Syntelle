package conversation

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestPrivateChannelParticipatesInShutdown(t *testing.T) {
	service := New(nil, nil, nil, nil)
	channel, err := service.OpenChannel(context.Background(), func(Event) {})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err = service.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-channel.Done():
	case <-ctx.Done():
		t.Fatal("private channel leaked")
	}
	if err = channel.Send(ctx, Event{Type: "hello"}); err == nil {
		t.Fatal("closed channel accepted input")
	}
	if _, err = service.OpenChannel(ctx, func(Event) {}); err == nil {
		t.Fatal("shutdown allowed a new channel")
	}
}

type literalChannelVoice struct {
	calls atomic.Int32
	text  string
}

func (v *literalChannelVoice) Prepare(_ context.Context, text, _, _, _ string) (any, error) {
	v.calls.Add(1)
	v.text = text
	return map[string]any{"audio_base64": []byte{1, 2}, "duration_ms": 1}, nil
}

func TestPrivateChannelLiteralSpeechBypassesModel(t *testing.T) {
	store := isolatedStore(t)
	voice := &literalChannelVoice{}
	service := New(store, nil, voice, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	events := make(chan Event, 32)
	channel, err := service.OpenChannel(ctx, func(e Event) {
		select {
		case events <- e:
		case <-ctx.Done():
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer channel.Close()
	defer service.Shutdown(ctx)
	read := func(kind string) Event {
		t.Helper()
		for {
			select {
			case e := <-events:
				if e.Type == "error" {
					t.Fatal(e.Text)
				}
				if e.Type == kind {
					return e
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
				return Event{}
			}
		}
	}
	cid := "private-channel-literal-session"
	if err = channel.Send(ctx, Event{Type: "hello", ConversationID: cid, TurnID: "literal-hello-0001", Character: "syntelle", Language: "en-US", Voice: "en-US-Ava:DragonHDLatestNeural"}); err != nil {
		t.Fatal(err)
	}
	read("ready")
	text := "Read this exactly.\n  Keep all words and punctuation!"
	input := Event{Type: "speak", ConversationID: cid, TurnID: "literal-input-0001", Text: text}
	if err = channel.Send(ctx, input); err != nil {
		t.Fatal(err)
	}
	prepared := read("prepared_speech")
	if prepared.Text != text || voice.text != text || voice.calls.Load() != 1 {
		t.Fatal("literal text changed or synthesized twice")
	}
	if err = channel.Send(ctx, input); err != nil {
		t.Fatal(err)
	}
	read("duplicate")
	if voice.calls.Load() != 1 {
		t.Fatal("duplicate literal command synthesized again")
	}
}
