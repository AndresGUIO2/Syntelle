package conversation

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"syntelle/render-contract"
	"testing"
	"time"
)

type overlappingPhraseSpeech struct {
	coordinatedSpeech
	started chan int
	finish  []chan struct{}
	pending atomic.Int32
	peak    atomic.Int32
}

type cuePhraseModel struct{ coordinatedModel }

func (m cuePhraseModel) CompleteWithPhrases(ctx context.Context, instructions string, messages []Message, emit func(PerformancePhrase) error) (ModelResult, error) {
	return m.coordinatedModel.CompleteWithPhrases(ctx, instructions, messages, func(p PerformancePhrase) error {
		amount := .7
		p.Cues = []PerformanceCue{{Kind: "emphasis", Anchor: "Complete thought", Amount: &amount}}
		return emit(p)
	})
}

func (s *overlappingPhraseSpeech) PrepareDirectedProgressive(ctx context.Context, text, language, voice, id string, d rendercontract.VoiceDelivery, publish func(any) error) (any, error) {
	value, err := s.PrepareDirected(ctx, text, language, voice, id, d)
	if err != nil {
		return nil, err
	}
	index := int(s.calls.Load()) - 1
	n := s.pending.Add(1)
	defer s.pending.Add(-1)
	for old := s.peak.Load(); n > old && !s.peak.CompareAndSwap(old, n); old = s.peak.Load() {
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case s.started <- index:
	}
	if err = publish(value); err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.finish[index]:
		return value, nil
	}
}

func TestNextPhrasePreparesAfterActivationBeforePreviousSynthesisCompletes(t *testing.T) {
	store := isolatedStore(t)
	voice := &overlappingPhraseSpeech{started: make(chan int, 3), finish: []chan struct{}{make(chan struct{}), make(chan struct{}), make(chan struct{})}}
	coordinator := &coordinatedCoordinator{}
	service := New(store, cuePhraseModel{coordinatedModel{count: 3}}, voice, nil)
	service.Delivery = coordinator
	service.Performance = &PerformanceProfile{MotionPolicy: rendercontract.ConversationalMotionVersion, MotionDirection: SemanticMotionDirection, Capabilities: []string{"emphasis"}}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	cid, id := "overlap_phrase_session", "overlap_phrase_turn"
	if _, err := store.Session(ctx, cid, "syntelle", "en-US", "offline fixture"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginTurn(ctx, cid, id, "question"); err != nil {
		t.Fatal(err)
	}
	events := make(chan Event, 40)
	done := make(chan struct{})
	go func() {
		defer close(done)
		service.runTurn(ctx, cid, id, "question", "en-US", rendercontract.DefaultVoice, "offline fixture", func(e Event) {
			select {
			case events <- e:
			case <-ctx.Done():
			}
		})
	}()
	t.Cleanup(func() { cancel(); <-done })
	first := true
	seen := map[int]bool{}
	finals := 0
	for {
		select {
		case <-ctx.Done():
			t.Fatal("overlapping phrase turn stalled")
		case e := <-events:
			switch e.Type {
			case "error":
				t.Fatal(e.Text)
			case "speech_prefix":
				part := e.Data.(SpeechPart)
				if part.Final {
					finals++
					continue
				}
				seen[part.Index] = true
				performance := part.Speech.(map[string]any)["performance"].(PerformanceValidation)
				if len(performance.Intent.Cues) != 1 || len(performance.Resolved) != 0 {
					t.Fatal("first PCM waited for an unresolved semantic cue", performance)
				}
				if part.Index > 0 && part.OffsetMS != float64(part.Index)*9000 {
					t.Fatal("unstable phrase offset", part.Index, part.OffsetMS)
				}
				coordinator.complete("activated")
				if first {
					first = false
					for want := 0; want < 2; want++ {
						select {
						case got := <-voice.started:
							if got != want {
								t.Fatal("preparation order", got, want)
							}
						case <-ctx.Done():
							t.Fatal("next synthesis waited for previous completion")
						}
					}
					if voice.calls.Load() != 2 || voice.peak.Load() != 2 {
						t.Fatal("lookahead exceeded one unactivated phrase", voice.calls.Load(), voice.peak.Load())
					}
					close(voice.finish[0])
				} else {
					close(voice.finish[part.Index])
				}
			case "speech_complete":
				<-done
				if len(seen) != 3 || finals != 3 || voice.pending.Load() != 0 {
					t.Fatal("incomplete or leaked phrase streams", seen, finals, voice.pending.Load())
				}
				return
			}
		}
	}
}

func TestSettingsInvalidateNextPreparationWaitingForPreviousLength(t *testing.T) {
	store := isolatedStore(t)
	voice := &overlappingPhraseSpeech{started: make(chan int, 3), finish: []chan struct{}{make(chan struct{}), make(chan struct{}), make(chan struct{})}}
	coordinator := &coordinatedCoordinator{}
	service := New(store, coordinatedModel{count: 2}, voice, nil)
	service.Delivery = coordinator
	service.Performance = &PerformanceProfile{MotionPolicy: rendercontract.ConversationalMotionVersion, MotionDirection: SemanticMotionDirection}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	cid, id := "settings_prefix_session", "settings_prefix_turn"
	if _, e := store.Session(ctx, cid, "syntelle", "en-US", "offline fixture"); e != nil {
		t.Fatal(e)
	}
	if _, e := store.BeginTurn(ctx, cid, id, "question"); e != nil {
		t.Fatal(e)
	}
	events := make(chan Event, 40)
	done := make(chan struct{})
	go func() {
		defer close(done)
		service.runTurn(ctx, cid, id, "question", "en-US", rendercontract.DefaultVoice, "offline fixture", func(e Event) {
			select {
			case events <- e:
			case <-ctx.Done():
			}
		})
	}()
	t.Cleanup(func() { cancel(); <-done })
	prefixes, finals := 0, 0
	for {
		select {
		case <-ctx.Done():
			t.Fatal("invalidated queued publisher did not release")
		case e := <-events:
			switch e.Type {
			case "error":
				t.Fatal(e.Text)
			case "speech_prefix":
				part := e.Data.(SpeechPart)
				if part.Final {
					finals++
					continue
				}
				prefixes++
				if part.Index == 0 {
					coordinator.complete("activated")
					for want := 0; want < 2; want++ {
						select {
						case got := <-voice.started:
							if got != want {
								t.Fatal(got, want)
							}
						case <-ctx.Done():
							t.Fatal("next preparation did not overlap")
						}
					}
					// The next publisher is blocked on the first recording's final
					// length. Its old settings must cancel without discarding audio 0.
					coordinator.complete("invalidated")
					close(voice.finish[0])
				} else {
					if part.OffsetMS != 9000 {
						t.Fatal("replacement lost phrase offset", part.OffsetMS)
					}
					coordinator.complete("activated")
					close(voice.finish[2])
				}
			case "speech_complete":
				<-done
				if prefixes != 2 || finals != 2 || voice.calls.Load() != 3 || voice.pending.Load() != 0 {
					t.Fatal("replacement replayed, lost or leaked audio", prefixes, finals, voice.calls.Load(), voice.pending.Load())
				}
				return
			}
		}
	}
}

func TestEveryOrdinaryPhraseCanPublishBeforeGestureResolution(t *testing.T) {
	s := &Service{Performance: &PerformanceProfile{MotionDirection: SemanticMotionDirection}}
	for _, index := range []int{0, 1, 7} {
		p := PerformancePhrase{Version: rendercontract.CoordinatedPerformanceVersion, Text: "This important thought can start playing now.", Cues: []PerformanceCue{{Kind: "emphasis", Anchor: "important thought"}}}
		if s.openingPublisher(t.Context(), "session", "turn", p, index, func(string, string, any) {}) == nil {
			t.Errorf("ordinary phrase %d with a proposed cue cannot stream", index)
		}
		p.Cues = append(p.Cues, PerformanceCue{Kind: "laugh", Anchor: p.Text})
		if s.openingPublisher(t.Context(), "session", "turn", p, index, func(string, string, any) {}) != nil {
			t.Errorf("laughter phrase %d lost its complete-recording path", index)
		}
	}
}

func TestLegacyMotionCanPlanGesturesBeforeThePhraseStarts(t *testing.T) {
	s := &Service{Performance: &PerformanceProfile{MotionPolicy: rendercontract.ConversationalMotionVersion}}
	p := PerformancePhrase{Version: rendercontract.CoordinatedPerformanceVersion, Text: "That is a useful point.", Cues: []PerformanceCue{{Kind: "emphasis", Anchor: "useful point"}}}
	publish := func(string, string, any) {}
	if s.openingPublisher(t.Context(), "session", "turn", p, 0, publish) != nil {
		t.Fatal("legacy cue was deferred past initial gesture admission")
	}
	p.Cues = nil
	if s.openingPublisher(t.Context(), "session", "turn", p, 0, publish) == nil || s.openingPublisher(t.Context(), "session", "turn", p, 1, publish) != nil {
		t.Fatal("legacy quiet opening or complete continuation changed")
	}
}

func TestProgressiveCueIdentityRetainsOriginalTimingAndSpendsOneCycle(t *testing.T) {
	first := ResolvedCue{ID: "p0-cue-0", Kind: "emphasis", Time: .5, Duration: .5, Peak: .75}
	changed := first
	changed.Time = 2
	changed.Peak = 2.25
	second := ResolvedCue{ID: "p0-cue-1", Kind: "brows", Time: 3, Duration: .5}
	merged := retainResolved([]ResolvedCue{first}, []ResolvedCue{changed, second})
	if len(merged) != 2 || merged[0].Time != first.Time || merged[0].Peak != first.Peak {
		t.Fatal("a growing prefix moved a resolved anchor", merged)
	}
	budget := newReplyGestures()
	validation := PerformanceValidation{Intent: PerformanceIntent{Version: rendercontract.CoordinatedPerformanceVersion}, Motion: &rendercontract.MotionTiming{Version: rendercontract.ConversationalMotionVersion, ReplyID: "reply"}, Resolved: []ResolvedCue{first}}
	for range 4 {
		if len(budget.admit(validation, 0).Resolved) != 1 {
			t.Fatal("an already admitted cue was lost")
		}
	}
	for i := 1; i < 8; i++ {
		cue := first
		cue.ID = fmt.Sprintf("p%d-cue-0", i)
		cue.Time = .5 + 2.3*float64(i)
		validation.Resolved = []ResolvedCue{cue}
		if len(budget.admit(validation, 0).Resolved) != 1 {
			t.Fatal("repeated prefixes spent extra rolling credits", i)
		}
	}
	validation.Resolved = []ResolvedCue{{ID: "p8-cue-0", Kind: "emphasis", Time: 18.9, Duration: .5}}
	if len(budget.admit(validation, 0).Resolved) != 0 {
		t.Fatal("rolling cycle ceiling was exceeded")
	}
}

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
