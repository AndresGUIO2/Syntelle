package conversation

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type voiceAdmissionFixture struct {
	active atomic.Int32
	block  chan struct{}
	reject bool
}

func (f *voiceAdmissionFixture) AcquireVoice(_ context.Context, _ string) (func(), error) {
	if f.reject {
		return nil, errors.New("full")
	}
	f.active.Add(1)
	var once sync.Once
	return func() { once.Do(func() { f.active.Add(-1) }) }, nil
}

func (f *voiceAdmissionFixture) Prepare(ctx context.Context, _, _, _, _ string) (any, error) {
	if f.block != nil {
		select {
		case <-f.block:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return map[string]any{"fixture": true}, nil
}

func TestVoiceAdmissionFollowsMicrophoneTurnAndPlayback(t *testing.T) {
	store := isolatedStore(t)
	voice := &voiceAdmissionFixture{block: make(chan struct{})}
	service := New(store, nil, voice, recognitionFixture{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	incoming := make(chan Event)
	outgoing := make(chan Event, 32)
	done := make(chan struct{})
	go func() {
		defer close(done)
		service.runSession(ctx, incoming, func(_ context.Context, event Event) { outgoing <- event })
	}()
	read := func(kind string) Event {
		t.Helper()
		for {
			select {
			case event := <-outgoing:
				if event.Type == kind {
					return event
				}
			case <-ctx.Done():
				t.Fatal("voice session did not emit", kind)
			}
		}
	}
	waitActive := func(want int32) {
		t.Helper()
		for voice.active.Load() != want {
			select {
			case <-ctx.Done():
				t.Fatalf("active voice users = %d, want %d", voice.active.Load(), want)
			case <-time.After(5 * time.Millisecond):
			}
		}
	}
	cid := "voice-admission-session-01"
	incoming <- Event{Type: "hello", ConversationID: cid, TurnID: "hello-admission-01", Character: "cafe", Language: "en-US", Voice: "en-US-Ava:DragonHDLatestNeural"}
	read("ready")
	incoming <- Event{Type: "mic_start", ConversationID: cid, TurnID: "mic-admission-01"}
	read("listening")
	waitActive(1)
	incoming <- Event{Type: "mic_stop", ConversationID: cid, TurnID: "mic-admission-01"}
	waitActive(0)
	incoming <- Event{Type: "speak", ConversationID: cid, TurnID: "turn-admission-01", Text: "Hello."}
	read("accepted")
	waitActive(1)
	incoming <- Event{Type: "playback_started", ConversationID: cid, TurnID: "turn-admission-01"}
	close(voice.block)
	read("prepared_speech")
	if voice.active.Load() != 1 {
		t.Fatal("voice slot was released during playback")
	}
	incoming <- Event{Type: "playback_ended", ConversationID: cid, TurnID: "turn-admission-01"}
	waitActive(0)
	voice.block = make(chan struct{})
	incoming <- Event{Type: "speak", ConversationID: cid, TurnID: "turn-admission-02", Text: "Hello again."}
	read("accepted")
	incoming <- Event{Type: "playback_started", ConversationID: cid, TurnID: "turn-admission-02"}
	close(voice.block)
	read("prepared_speech")
	time.Sleep(20 * time.Millisecond)
	incoming <- Event{Type: "cancel", ConversationID: cid, TurnID: "turn-admission-02", Reason: "owner stopped reply"}
	waitActive(0)
	cancel()
	<-done
}

func TestVoiceCapacityRejectionStopsMicrophone(t *testing.T) {
	store := isolatedStore(t)
	voice := &voiceAdmissionFixture{reject: true}
	service := New(store, nil, voice, recognitionFixture{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	incoming := make(chan Event, 4)
	outgoing := make(chan Event, 8)
	done := make(chan struct{})
	go func() {
		defer close(done)
		service.runSession(ctx, incoming, func(_ context.Context, event Event) { outgoing <- event })
	}()
	cid := "voice-rejected-session-01"
	incoming <- Event{Type: "hello", ConversationID: cid, TurnID: "hello-rejected-01", Character: "cafe", Language: "en-US", Voice: "en-US-Ava:DragonHDLatestNeural"}
	select {
	case event := <-outgoing:
		if event.Type != "ready" {
			t.Fatal(event.Type)
		}
	case <-ctx.Done():
		t.Fatal("conversation did not start")
	}
	incoming <- Event{Type: "mic_start", ConversationID: cid, TurnID: "mic-rejected-01"}
	select {
	case event := <-outgoing:
		if event.Type != "microphone_error" || event.TurnID != "mic-rejected-01" || voice.active.Load() != 0 {
			t.Fatal("capacity rejection was not sent to the microphone", event)
		}
	case <-ctx.Done():
		t.Fatal("microphone admission did not answer")
	}
	cancel()
	<-done
}
