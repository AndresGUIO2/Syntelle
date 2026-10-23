package speech

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeSpeechProvider struct {
	calls     int
	missing   bool
	cancelled bool
}

func (p *fakeSpeechProvider) Synthesize(ctx context.Context, input Input) (Result, error) {
	p.calls++
	if p.cancelled {
		<-ctx.Done()
		return Result{}, ctx.Err()
	}
	result := Result{Audio: make([]byte, 44), Mime: "audio/wav", DurationMS: 1000, Visemes: []Viseme{{0, 0}, {2, 100}, {21, 500}, {0, 900}}}
	if p.missing {
		result.Visemes = nil
	}
	return result, nil
}

type fakeSpeechLedger struct {
	saved        *Result
	failure      string
	halted       bool
	reserveError bool
	finished     bool
}

func (l *fakeSpeechLedger) Reserve(_ context.Context, _ Input, _ string, _ int) (int64, *Result, error) {
	if l.reserveError {
		return 0, nil, errors.New("Database unavailable.")
	}
	return 1, l.saved, nil
}
func (l *fakeSpeechLedger) Finish(_ context.Context, _ int64, result *Result, message string, halt bool) error {
	l.finished = true
	l.failure = message
	l.halted = halt
	if message == "" {
		l.saved = result
	}
	return nil
}
func validSpeechInput() Input {
	return Input{Text: "Hello.", Language: "en-US", Voice: "en-US-Ava:DragonHDLatestNeural", RequestKey: "test-request-01"}
}
func postSpeech(handler http.Handler, input Input) *httptest.ResponseRecorder {
	body, _ := json.Marshal(input)
	request := httptest.NewRequest("POST", "/api/v1/speech/synthesize", bytes.NewReader(body))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
func TestSpeechValidatedBeforePaidCallAndCached(t *testing.T) {
	provider := &fakeSpeechProvider{}
	ledger := &fakeSpeechLedger{}
	handler := Handler(http.NotFoundHandler(), &Service{provider, ledger, "eastus"})
	invalid := validSpeechInput()
	invalid.Voice = "other"
	if postSpeech(handler, invalid).Code != 400 || provider.calls != 0 {
		t.Fatal("Invalid request reached provider")
	}
	if response := postSpeech(handler, validSpeechInput()); response.Code != 200 || !ledger.finished {
		t.Fatal(response.Code, "Response preceded archive")
	}
	if postSpeech(handler, validSpeechInput()).Code != 200 || provider.calls != 1 {
		t.Fatal("Cached speech called provider")
	}
}
func TestSpeechMissingEventsHaltsWithoutFallback(t *testing.T) {
	provider := &fakeSpeechProvider{missing: true}
	ledger := &fakeSpeechLedger{}
	response := postSpeech(Handler(http.NotFoundHandler(), &Service{provider, ledger, "eastus"}), validSpeechInput())
	if response.Code != 502 || !ledger.halted || ledger.failure == "" || provider.calls != 1 {
		t.Fatal("Missing native events were accepted")
	}
}
func TestSpeechDatabaseFailurePreventsProvider(t *testing.T) {
	provider := &fakeSpeechProvider{}
	ledger := &fakeSpeechLedger{reserveError: true}
	response := postSpeech(Handler(http.NotFoundHandler(), &Service{provider, ledger, "eastus"}), validSpeechInput())
	if response.Code != 409 || provider.calls != 0 {
		t.Fatal("Provider ran without committed input")
	}
}

func TestSpeechCancelledRequestIsRecordedWithoutProviderFallback(t *testing.T) {
	provider := &fakeSpeechProvider{cancelled: true}
	ledger := &fakeSpeechLedger{}
	body, _ := json.Marshal(validSpeechInput())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest("POST", "/api/v1/speech/synthesize", bytes.NewReader(body)).WithContext(ctx)
	response := httptest.NewRecorder()
	Handler(http.NotFoundHandler(), &Service{provider, ledger, "canadacentral"}).ServeHTTP(response, request)
	if response.Code != 502 || !ledger.finished || ledger.halted || provider.calls != 1 || ledger.failure != "Speech request cancelled." {
		t.Fatal("Cancellation did not preserve the terminal attempt")
	}
}

func TestSpeechRejectsTrailingJSONBeforeReservation(t *testing.T) {
	provider := &fakeSpeechProvider{}
	ledger := &fakeSpeechLedger{}
	body, _ := json.Marshal(validSpeechInput())
	request := httptest.NewRequest("POST", "/api/v1/speech/synthesize", bytes.NewReader(append(body, []byte(" {}")...)))
	response := httptest.NewRecorder()
	Handler(http.NotFoundHandler(), &Service{provider, ledger, "canadacentral"}).ServeHTTP(response, request)
	if response.Code != 400 || provider.calls != 0 {
		t.Fatal("Multiple request objects reached the provider")
	}
}
func TestSpeechDisabledAndTimelineValidation(t *testing.T) {
	if postSpeech(Handler(http.NotFoundHandler(), nil), validSpeechInput()).Code != 503 {
		t.Fatal("Unconfigured speech enabled")
	}
	for _, events := range [][]Viseme{{{2, 0}, {1, -1}}, {{2, 0}, {25, 1}}, {{0, 0}, {0, 10}}} {
		if validateSpeechResult(Result{Audio: make([]byte, 44), DurationMS: 1000, Visemes: events}) == nil {
			t.Fatal("Invalid events accepted")
		}
	}
	input := validSpeechInput()
	input.Text = string(bytes.Repeat([]byte("a"), 501))
	if validateSpeech(input) == nil {
		t.Fatal("Unbounded speech")
	}
}
