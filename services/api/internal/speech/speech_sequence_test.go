package speech

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"strings"
	"sync"
	"syntelle/api/internal/conversation"
	"testing"
	"unicode/utf16"
)

func sequenceFixtureWAV(sample int16) []byte {
	wav := make([]byte, 44+4800)
	copy(wav, "RIFF")
	binary.LittleEndian.PutUint32(wav[4:], uint32(len(wav)-8))
	copy(wav[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(wav[16:], 16)
	binary.LittleEndian.PutUint16(wav[20:], 1)
	binary.LittleEndian.PutUint16(wav[22:], 1)
	binary.LittleEndian.PutUint32(wav[24:], 24000)
	binary.LittleEndian.PutUint32(wav[28:], 48000)
	binary.LittleEndian.PutUint16(wav[32:], 2)
	binary.LittleEndian.PutUint16(wav[34:], 16)
	copy(wav[36:], "data")
	binary.LittleEndian.PutUint32(wav[40:], 4800)
	for at := 44; at < len(wav); at += 2 {
		binary.LittleEndian.PutUint16(wav[at:], uint16(sample))
	}
	return wav
}

func TestProgressiveSpeechStartsWithOpeningPhraseAndRetainsNativeParts(t *testing.T) {
	text := "This is a complete opening phrase. " + strings.Repeat("A detailed explanation with Unicode 🙂. ", 90)
	provider := &sequenceProviderFixture{}
	service := NewService(provider, &sequenceLedgerFixture{}, "canadacentral")
	speech := NewConversationSpeech(service).(conversationSpeech)
	count := 0
	var delivered strings.Builder
	prepared, err := speech.PrepareParts(context.Background(), text, "en-US", "en-US-Ava:DragonHDLatestNeural", "progressive-fixture", func(part conversation.SpeechPart) error {
		if part.Index != count || part.OffsetMS != float64(count)*100 {
			t.Fatal("not delivered incrementally", part)
		}
		if count == 0 && part.Text != "This is a complete opening phrase. " {
			t.Fatal("opening phrase was buffered", part.Text)
		}
		count++
		delivered.WriteString(part.Text)
		return nil
	})
	if err != nil || count < 2 || delivered.String() != text {
		t.Fatal(count, err)
	}
	if prepared.(Result).DurationMS != float64(count)*100 {
		t.Fatal("native clock lost")
	}
	for _, input := range []string{strings.Repeat("🙂", 2000), strings.Repeat("x", 3000), "Short.", "Hi. A longer opening phrase follows. Final sentence."} {
		if strings.Join(splitProgressiveSpeech(input), "") != input {
			t.Fatal("split lost text")
		}
	}
	provider.texts = nil
	_, err = speech.PrepareParts(context.Background(), text, "en-US", "en-US-Ava:DragonHDLatestNeural", "progressive-stopped", func(conversation.SpeechPart) error { return context.Canceled })
	if !errors.Is(err, context.Canceled) || len(provider.texts) > 2 {
		t.Fatal("continued after stop", err)
	}
}

func TestSpeechSplitPreservesCompleteTextAndUnicode(t *testing.T) {
	for _, text := range []string{strings.Repeat("A full sentence. ", 600), strings.Repeat("🙂", 4000), strings.Repeat("x", 12001), "A complete short reply."} {
		chunks := splitSpeech(text)
		if strings.Join(chunks, "") != text {
			t.Fatal("text lost between speech parts")
		}
		for _, chunk := range chunks {
			if len(utf16.Encode([]rune(chunk))) > conversationPartUnits || strings.TrimSpace(chunk) == "" {
				t.Fatal("invalid part")
			}
		}
	}
}

type sequenceProviderFixture struct {
	mu     sync.Mutex
	texts  []string
	failAt int
	cancel context.CancelFunc
}

func (p *sequenceProviderFixture) Synthesize(_ context.Context, input Input) (Result, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.texts = append(p.texts, input.Text)
	if len(p.texts) == p.failAt {
		return Result{}, errors.New("fixture error")
	}
	if p.cancel != nil {
		p.cancel()
	}
	return Result{Audio: sequenceFixtureWAV(int16(len(p.texts) * 100)), DurationMS: 100, Visemes: []Viseme{{0, 0}, {2, 20}, {0, 100}}, ProviderResultID: "native-fixture", Mime: "audio/wav"}, nil
}

type sequenceLedgerFixture struct{ cache map[string]Result }

func (l *sequenceLedgerFixture) Reserve(_ context.Context, _ Input, binding string, _ int) (int64, *Result, error) {
	if r, ok := l.cache[binding]; ok {
		return 0, &r, nil
	}
	return 1, nil, nil
}
func (l *sequenceLedgerFixture) Finish(context.Context, int64, *Result, string, bool) error {
	return nil
}

func TestConversationSpeechReadsEntireReplyAsOneTimeline(t *testing.T) {
	provider := &sequenceProviderFixture{}
	service := NewService(provider, &sequenceLedgerFixture{}, "canadacentral")
	text := strings.Repeat("A detailed explanation of the topic. ", 200)
	prepared, err := NewConversationSpeech(service).Prepare(context.Background(), text, "en-US", "en-US-Ava:DragonHDLatestNeural", "long-turn-fixture")
	if err != nil {
		t.Fatal(err)
	}
	result := prepared.(Result)
	if strings.Join(provider.texts, "") != text || len(result.Parts) < 2 {
		t.Fatal("reply omitted speech")
	}
	if result.DurationMS != float64(len(result.Parts))*100 || result.Visemes[len(result.Visemes)-1].OffsetMS != result.DurationMS {
		t.Fatal("combined clock differs", result.DurationMS)
	}
	_, pcm, err := speechPCM(result.Audio)
	if err != nil {
		t.Fatal(err)
	}
	for i := range result.Parts {
		_, original, _ := speechPCM(sequenceFixtureWAV(int16((i + 1) * 100)))
		if !bytes.Equal(pcm[i*4800:(i+1)*4800], original) {
			t.Fatal("native audio changed")
		}
	}
	short := strings.Repeat("A useful explanation. ", 40)
	if _, err = NewConversationSpeech(service).Prepare(context.Background(), short, "en-US", "en-US-Ava:DragonHDLatestNeural", "medium-turn-fixture"); err != nil {
		t.Fatal("reply over 500 rejected", err)
	}
	input := validSpeechInput()
	input.Text = short
	if validateSpeech(input) == nil {
		t.Fatal("manual trial input policy changed")
	}
}

func TestSpeechSequenceStopsAtCancellationOrProviderFailure(t *testing.T) {
	for _, cancelled := range []bool{true, false} {
		ctx, cancel := context.WithCancel(context.Background())
		provider := &sequenceProviderFixture{failAt: 1}
		if cancelled {
			provider.failAt = 0
			provider.cancel = cancel
		}
		service := NewService(provider, &sequenceLedgerFixture{}, "canadacentral")
		_, err := NewConversationSpeech(service).Prepare(ctx, strings.Repeat("An explanation. ", 500), "en-US", "en-US-Ava:DragonHDLatestNeural", "cancel-long-fixture")
		cancel()
		if err == nil || len(provider.texts) != 1 {
			t.Fatal("preparation continued or retried after stop", err, len(provider.texts))
		}
	}
}
