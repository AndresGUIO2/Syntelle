package speech

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"regexp"
	"strings"
	"syntelle/api/internal/httpapi"
	"time"
	"unicode/utf16"
)

const speechSDKVersion = "1.52.0"
const speechNativeSHA256 = "2573d5336cdba31759967b77b3596dcd450b802fa8c1a410245c541a5df0fb47"

var speechVoices = map[string]bool{"en-US-Ava:DragonHDLatestNeural": true, "en-US-Andrew:DragonHDLatestNeural": true}
var speechKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{8,128}$`)

type Input struct {
	Text       string `json:"text"`
	Language   string `json:"language"`
	Voice      string `json:"voice"`
	RequestKey string `json:"request_key"`
	region     string
}
type Viseme struct {
	ID       uint    `json:"id"`
	OffsetMS float64 `json:"offset_ms"`
}
type Result struct {
	Parts            []PreparedPart `json:"parts,omitempty"`
	Region           string         `json:"region,omitempty"`
	Audio            []byte         `json:"audio_base64"`
	Mime             string         `json:"mime"`
	DurationMS       float64        `json:"duration_ms"`
	Visemes          []Viseme       `json:"visemes"`
	Voice            string         `json:"voice"`
	Language         string         `json:"language"`
	SDKVersion       string         `json:"sdk_version"`
	ProviderResultID string         `json:"provider_result_id"`
	ProviderStatus   string         `json:"provider_status"`
	Cached           bool           `json:"cached"`
}
type Provider interface {
	Synthesize(context.Context, Input) (Result, error)
}
type Ledger interface {
	Reserve(context.Context, Input, string, int) (int64, *Result, error)
	Finish(context.Context, int64, *Result, string, bool) error
}
type Service struct {
	provider Provider
	ledger   Ledger
	region   string
}

func NewService(provider Provider, ledger Ledger, region string) *Service {
	return &Service{provider: provider, ledger: ledger, region: region}
}

func validateSpeech(input Input) error {
	return validateSpeechWithLimit(input, 500)
}
func validateSpeechWithLimit(input Input, limit int) error {
	count := len(utf16.Encode([]rune(input.Text)))
	if strings.TrimSpace(input.Text) == "" || count > limit || !speechKeyPattern.MatchString(input.RequestKey) {
		return fmt.Errorf("provide plain text of 1–%d characters and an idempotency key", limit)
	}
	if !speechVoices[input.Voice] || (input.Language != "en-US" && input.Language != "es-CO") {
		return errors.New("Choose an allowed Dragon HD voice and English or Spanish.")
	}
	return nil
}
func validateSpeechResult(result Result) error {
	if len(result.Audio) < 44 || result.DurationMS <= 0 || len(result.Visemes) < 2 {
		return errors.New("Dragon HD did not return usable audio and native viseme events.")
	}
	previous := -1.
	spoken := false
	for _, event := range result.Visemes {
		if event.ID > 21 || event.OffsetMS < previous || event.OffsetMS < 0 || event.OffsetMS > result.DurationMS+100 {
			return errors.New("Invalid native viseme timeline.")
		}
		previous = event.OffsetMS
		spoken = spoken || event.ID > 0
	}
	if !spoken {
		return errors.New("Dragon HD returned no spoken native visemes.")
	}
	return nil
}

func Handler(next http.Handler, service *Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/speech/status" && r.Method == http.MethodGet {
			httpapi.WriteJSON(w, 200, map[string]any{"configured": service != nil, "voices": []string{"en-US-Ava:DragonHDLatestNeural", "en-US-Andrew:DragonHDLatestNeural"}, "max_characters": 500})
			return
		}
		if r.URL.Path != "/api/v1/speech/synthesize" {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodPost {
			httpapi.WriteError(w, 405, "method_not_allowed", "Use POST.")
			return
		}
		if service == nil {
			httpapi.WriteError(w, 503, "speech_not_configured", "Configure Azure Speech on the local server to enable Dragon HD.")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 8192)
		defer r.Body.Close()
		var input Input
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			httpapi.WriteError(w, 400, "invalid_speech_request", "Send a valid speech request.")
			return
		}
		if decoder.Decode(new(any)) != io.EOF {
			httpapi.WriteError(w, 400, "invalid_speech_request", "Send exactly one speech request.")
			return
		}
		if err := validateSpeech(input); err != nil {
			httpapi.WriteError(w, 400, "invalid_speech_request", err.Error())
			return
		}
		result, err := service.synthesize(r.Context(), input)
		if err != nil {
			var failure *speechFailure
			if errors.As(err, &failure) {
				httpapi.WriteError(w, failure.status, failure.code, failure.message)
			} else {
				httpapi.WriteError(w, 502, "speech_failed", err.Error())
			}
			return
		}
		httpapi.WriteJSON(w, 200, result)
	})
}

type speechFailure struct {
	status        int
	code, message string
}

func (e *speechFailure) Error() string { return e.message }
func (service *Service) synthesize(ctx context.Context, input Input) (Result, error) {
	return service.synthesizeBounded(ctx, input, 500, 20*time.Second)
}
func (service *Service) synthesizeBounded(ctx context.Context, input Input, limit int, timeout time.Duration) (Result, error) {
	if err := validateSpeechWithLimit(input, limit); err != nil {
		return Result{}, err
	}
	bindingBytes, _ := json.Marshal(struct{ Text, Language, Voice, Region, SDK, Format string }{input.Text, input.Language, input.Voice, service.region, speechSDKVersion, "riff-24khz-16bit-mono-pcm"})
	hash := sha256.Sum256(bindingBytes)
	input.region = service.region
	id, cached, err := service.ledger.Reserve(ctx, input, hex.EncodeToString(hash[:]), len(utf16.Encode([]rune(input.Text))))
	if err != nil {
		return Result{}, &speechFailure{409, "speech_reservation_failed", err.Error()}
	}
	if cached != nil {
		cached.Cached = true
		cached.Region = service.region
		return *cached, nil
	}
	providerContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	result, err := service.provider.Synthesize(providerContext, input)
	if err == nil {
		err = validateSpeechResult(result)
	}
	if err != nil {
		cancelled := ctx.Err() != nil
		message := "Azure speech preparation failed; the attempt is recorded and will not be retried."
		if cancelled {
			message = "Speech request cancelled."
		}
		if e := service.ledger.Finish(context.WithoutCancel(ctx), id, &result, message, !cancelled); e != nil {
			return result, &speechFailure{503, "speech_archive_failed", "Speech evidence could not be committed."}
		}
		return result, &speechFailure{502, "speech_failed", message}
	}
	result.Voice = input.Voice
	result.Region = service.region
	result.Language = input.Language
	result.SDKVersion = speechSDKVersion
	if err = service.ledger.Finish(context.WithoutCancel(ctx), id, &result, "", false); err != nil {
		return result, &speechFailure{503, "speech_archive_failed", "Speech evidence could not be committed; no retry will run."}
	}
	return result, nil
}
