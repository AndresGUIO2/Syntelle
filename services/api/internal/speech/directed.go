package speech

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"math"
	"os"
	"strings"
	"syntelle/render-contract"
	"unicode/utf16"
)

type LocaleCapabilities struct {
	Available bool     `json:"available"`
	Styles    []string `json:"styles"`
}
type VoiceCapabilities struct {
	Version  string                        `json:"version"`
	Revision string                        `json:"revision"`
	Voice    string                        `json:"voice"`
	Region   string                        `json:"region"`
	Locales  map[string]LocaleCapabilities `json:"locales"`
}

func (s *Service) LoadVoiceCapabilities(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return errors.New("voice capabilities could not be read")
	}
	var c VoiceCapabilities
	if json.Unmarshal(b, &c) != nil || c.Version != "voice-capabilities/1" || c.Revision == "" || !rendercontract.SupportedVoice(c.Voice) || c.Region != s.region {
		return errors.New("voice capability identity mismatch")
	}
	for locale, v := range c.Locales {
		if locale != "en-US" && locale != "es-CO" {
			return errors.New("invalid voice capability locale")
		}
		for _, style := range v.Styles {
			if style != "reassuring" && style != "curious" && style != "amused" {
				return errors.New("invalid voice style capability")
			}
		}
	}
	s.capabilities = &c
	return nil
}

func (s *Service) VoiceDirection(r rendercontract.DeliveryReservation) (rendercontract.VoiceDelivery, error) {
	if !r.Valid() {
		return rendercontract.VoiceDelivery{}, errors.New("invalid approved delivery")
	}
	d := rendercontract.VoiceDelivery{Version: rendercontract.DeliveryVersion, Purpose: r.Request.Purpose, Emotion: r.Effective, CapabilityRevision: "plain-contextual/1"}
	if s.capabilities == nil {
		return d, nil
	}
	c := s.capabilities
	if r.Request.Voice != c.Voice || !c.Locales[r.Request.Language].Available {
		return d, errors.New("selected voice is not qualified for this language")
	}
	d.CapabilityRevision = c.Revision
	if !r.Expressive {
		return d, nil
	}
	style := ""
	switch r.Request.Purpose {
	case "reassure":
		if r.Effective.State != "angry" && r.Effective.State != "disgusted" {
			style = "reassuring"
		}
	case "ask":
		if r.Effective.State != "sad" && r.Effective.State != "angry" && r.Effective.State != "fearful" && r.Effective.State != "disgusted" {
			style = "curious"
		}
	case "playful":
		if r.Effective.State == "playful" || r.Effective.State == "joyful" {
			style = "amused"
		}
	}
	for _, allowed := range c.Locales[r.Request.Language].Styles {
		if style == allowed {
			d.Style = style
		}
	}
	return d, nil
}

// ValidateResult is shared by bounded offline probes and normal preparation.
func ValidateResult(result Result) error { return validateSpeechResult(result) }

// Laughter uses its prepared facial path and may legitimately emit only Rest.
// Keep normal spoken-phoneme validation unchanged.
func ValidateLaughterResult(result Result) error {
	if len(result.Audio) < 44 || result.DurationMS <= 0 || result.DurationMS > 2500 || math.IsNaN(result.DurationMS) || math.IsInf(result.DurationMS, 0) || len(result.Visemes) < 2 {
		return errors.New("invalid bounded laughter audio/events")
	}
	previous := -1.
	for _, v := range result.Visemes {
		if v.ID > 21 || v.OffsetMS < 0 || v.OffsetMS < previous || v.OffsetMS > result.DurationMS+maxVisemeTailMS || math.IsNaN(v.OffsetMS) || math.IsInf(v.OffsetMS, 0) {
			return errors.New("invalid native laughter timeline")
		}
		previous = v.OffsetMS
	}
	return nil
}

// WithVoiceDelivery is available only to trusted in-process adapters/probes.
// JSON decoding Input cannot set either of its private provider fields.
func WithVoiceDelivery(input Input, d rendercontract.VoiceDelivery) (Input, error) {
	if !d.Valid() {
		return input, errors.New("invalid voice delivery")
	}
	copy := d
	input.delivery = &copy
	if d.Style != "" {
		if !rendercontract.SupportedVoice(input.Voice) || (input.Language != "en-US" && input.Language != "es-CO") {
			return input, errors.New("invalid directed voice identity")
		}
		var escaped bytes.Buffer
		if err := xml.EscapeText(&escaped, []byte(input.Text)); err != nil {
			return input, err
		}
		input.providerText = `<speak version="1.0" xmlns="http://www.w3.org/2001/10/synthesis" xmlns:mstts="https://www.w3.org/2001/mstts" xml:lang="` + input.Language + `"><voice name="` + input.Voice + `"><mstts:express-as style="` + d.Style + `">` + escaped.String() + `</mstts:express-as></voice></speak>`
	}
	return input, nil
}

func (i Input) MarshalJSON() ([]byte, error) {
	type plain Input
	if i.providerText == "" {
		return json.Marshal(plain(i))
	}
	return json.Marshal(struct {
		plain
		Delivery      *rendercontract.VoiceDelivery `json:"delivery"`
		ProviderInput string                        `json:"provider_input"`
		Serializer    string                        `json:"serializer"`
	}{plain(i), i.delivery, i.providerText, "azure-ssml/1"})
}

func speechBinding(input Input, region string) string {
	// Keep the historical plain binding byte-for-byte compatible.
	b, _ := json.Marshal(struct{ Text, Language, Voice, Region, SDK, Format string }{input.Text, input.Language, input.Voice, region, speechSDKVersion, "riff-24khz-16bit-mono-pcm"})
	if input.providerText != "" {
		b, _ = json.Marshal(struct {
			Plain                     json.RawMessage
			Delivery                  *rendercontract.VoiceDelivery
			ProviderInput, Serializer string
		}{b, input.delivery, input.providerText, "azure-ssml/1"})
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func (i Input) ProviderInput() string {
	if i.providerText != "" {
		return i.providerText
	}
	return i.Text
}

func normalizeWords(input Input, words []rendercontract.WordBoundary, duration float64) []rendercontract.WordBoundary {
	clean := utf16.Encode([]rune(input.Text))
	provider := input.ProviderInput()
	previousOffset, previousTime := 0, -1.
	result := make([]rendercontract.WordBoundary, 0, len(words))
	for _, w := range words {
		if w.Text == "" || w.DurationMS <= 0 {
			continue
		}
		if math.IsNaN(w.OffsetMS) || math.IsInf(w.OffsetMS, 0) || math.IsNaN(w.DurationMS) || math.IsInf(w.DurationMS, 0) || w.OffsetMS < previousTime || w.OffsetMS < 0 || w.OffsetMS+w.DurationMS > duration+2 {
			return nil
		}
		match := -1
		for start := previousOffset; start < len(clean); start++ {
			units := utf16.Encode([]rune(w.Text))
			end := start + len(units)
			if end > len(clean) || string(utf16.Decode(clean[start:end])) != w.Text {
				continue
			}
			offset := start
			if input.providerText != "" {
				var escaped bytes.Buffer
				_ = xml.EscapeText(&escaped, []byte(input.Text))
				prefix := strings.Index(provider, escaped.String())
				var before bytes.Buffer
				_ = xml.EscapeText(&before, []byte(string(utf16.Decode(clean[:start]))))
				rawOffset := len(utf16.Encode([]rune(provider[:prefix] + before.String())))
				if w.TextOffset != offset && w.TextOffset != rawOffset {
					continue
				}
			} else if w.TextOffset != offset {
				continue
			}
			if match >= 0 {
				return nil
			}
			match = start
		}
		if match < 0 {
			return nil
		}
		w.TextOffset = match
		w.Length = len(utf16.Encode([]rune(w.Text)))
		result = append(result, w)
		previousOffset = match + w.Length
		previousTime = w.OffsetMS
	}
	return result
}
