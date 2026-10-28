package speech

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"syntelle/render-contract"
	"testing"
)

func TestDefaultVoiceIsAcceptedWithoutAllowingPublicProviderMarkup(t *testing.T) {
	i := validSpeechInput()
	i.Voice = rendercontract.DefaultVoice
	if err := validateSpeech(i); err != nil {
		t.Fatal(err)
	}
	var public Input
	if err := json.Unmarshal([]byte(`{"text":"Hello.","language":"en-US","voice":"en-US-Serena:DragonHDLatestNeural","request_key":"valid-key-123","provider_text":"[angry]","style":"angry"}`), &public); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(public)
	if strings.Contains(string(b), "angry") {
		t.Fatal("public input injected provider instructions")
	}
}

func TestPlainSpeechRejectsInlineProviderTagsBeforeCallingAzure(t *testing.T) {
	for _, text := range []string{"[angry] Hello.", "<speak>Hello.</speak>"} {
		t.Run(text, func(t *testing.T) {
			p := &fakeSpeechProvider{}
			s := NewService(p, &fakeSpeechLedger{}, "canadacentral")
			i := validSpeechInput()
			i.Text = text
			if _, err := s.synthesize(t.Context(), i); err == nil || p.calls != 0 {
				t.Fatal("public text reached provider with markup", err, p.calls)
			}
		})
	}
}

func TestNonverbalRestEventsDoNotWeakenSpokenArticulationGate(t *testing.T) {
	r := Result{Audio: make([]byte, 44), DurationMS: 800, Visemes: []Viseme{{ID: 0, OffsetMS: 0}, {ID: 0, OffsetMS: 400}}}
	if ValidateLaughterResult(r) != nil || ValidateResult(r) == nil {
		t.Fatal("nonverbal and speech gates were conflated")
	}
	r.Visemes[1].OffsetMS = -1
	if ValidateLaughterResult(r) == nil {
		t.Fatal("invalid nonverbal timeline admitted")
	}
	r.Visemes[1].OffsetMS = 400
	r.DurationMS = 2600
	if ValidateLaughterResult(r) == nil {
		t.Fatal("laughter duration gate weakened")
	}
}

func TestDirectedBindingPreservesPlainAndSeparatesStyles(t *testing.T) {
	i := validSpeechInput()
	i.Voice = rendercontract.DefaultVoice
	b, _ := json.Marshal(struct{ Text, Language, Voice, Region, SDK, Format string }{i.Text, i.Language, i.Voice, "canadacentral", speechSDKVersion, "riff-24khz-16bit-mono-pcm"})
	want := fmt.Sprintf("%x", sha256.Sum256(b))
	if speechBinding(i, "canadacentral") != want {
		t.Fatal("historical plain binding changed")
	}
	d := rendercontract.VoiceDelivery{Version: rendercontract.DeliveryVersion, Purpose: "reassure", Emotion: rendercontract.EmotionTarget{State: "neutral", Energy: .5}, CapabilityRevision: "test/1"}
	plain, err := WithVoiceDelivery(i, d)
	if err != nil {
		t.Fatal(err)
	}
	if speechBinding(plain, "canadacentral") != want {
		t.Fatal("contextual plain input lost its cache")
	}
	d.Style = "reassuring"
	first, err := WithVoiceDelivery(i, d)
	if err != nil {
		t.Fatal(err)
	}
	d.Style = "curious"
	second, err := WithVoiceDelivery(i, d)
	if err != nil {
		t.Fatal(err)
	}
	if speechBinding(first, "canadacentral") == want || speechBinding(first, "canadacentral") == speechBinding(second, "canadacentral") {
		t.Fatal("different delivery shared a binding")
	}
	i.Text = `Hello & <voice name="other">world</voice>.`
	escaped, err := WithVoiceDelivery(i, d)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(escaped.ProviderInput(), `<voice name="other">`) || !strings.Contains(escaped.ProviderInput(), "&lt;voice") {
		t.Fatal("spoken text escaped provider serialization")
	}
}

func TestDirectedFailureMakesOneCallAndNoFallback(t *testing.T) {
	p := &fakeSpeechProvider{missing: true}
	l := &fakeSpeechLedger{}
	s := NewService(p, l, "canadacentral")
	d := rendercontract.VoiceDelivery{Version: rendercontract.DeliveryVersion, Purpose: "ask", Emotion: rendercontract.EmotionTarget{State: "curious", Intensity: .2, Energy: .5}, Style: "curious", CapabilityRevision: "fixture/1"}
	i, err := WithVoiceDelivery(validSpeechInput(), d)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.synthesize(context.Background(), i); err == nil || p.calls != 1 || !l.halted {
		t.Fatal("directed failure retried or silently succeeded", p.calls, err)
	}
}

func TestWordOffsetsAreCheckedAgainstCleanUTF16Text(t *testing.T) {
	i := validSpeechInput()
	i.Text = "Hi 🙂 & hello."
	w := []rendercontract.WordBoundary{{Text: "hello", TextOffset: 8, OffsetMS: 200, DurationMS: 100}}
	got := normalizeWords(i, w, 1000)
	if len(got) != 1 || got[0].TextOffset != 8 {
		t.Fatal(got)
	}
	w[0].TextOffset = 2
	if len(normalizeWords(i, w, 1000)) != 0 {
		t.Fatal("invalid word mapping admitted")
	}
}

func TestVoiceCapabilitiesArePerLanguageAndRespectApprovedEmotion(t *testing.T) {
	s := NewService(&fakeSpeechProvider{}, &fakeSpeechLedger{}, "canadacentral")
	s.capabilities = &VoiceCapabilities{Version: "voice-capabilities/1", Revision: "fixture/1", Voice: rendercontract.DefaultVoice, Region: "canadacentral", Locales: map[string]LocaleCapabilities{"en-US": {Available: true, Styles: []string{"reassuring", "curious", "amused"}}, "es-CO": {Available: true}}}
	r := rendercontract.DeliveryReservation{Expressive: true, Version: rendercontract.DeliveryVersion, Token: "fixture_token", Instance: "fixture_instance", Effective: rendercontract.EmotionTarget{State: "curious", Intensity: .3, Energy: .5}, Request: rendercontract.DeliveryRequest{RequestID: "fixture_request", TurnID: "fixture_turn", PhraseIndex: 0, Revision: 1, TextHash: rendercontract.TextHash("Hello."), Voice: rendercontract.DefaultVoice, Language: "en-US", Purpose: "ask"}}
	d, err := s.VoiceDirection(r)
	if err != nil || d.Style != "curious" {
		t.Fatal(d, err)
	}
	r.Request.Language = "es-CO"
	d, err = s.VoiceDirection(r)
	if err != nil || d.Style != "" {
		t.Fatal("unqualified Spanish style activated", d, err)
	}
	r.Request.Language = "en-US"
	r.Effective.State = "angry"
	d, err = s.VoiceDirection(r)
	if err != nil || d.Style != "" {
		t.Fatal("voice contradicted approved mood", d, err)
	}
	r.Effective.State = "curious"
	r.Expressive = false
	d, err = s.VoiceDirection(r)
	if err != nil || d.Style != "" {
		t.Fatal("automatic delivery ignored its toggle", d, err)
	}
}

func TestVoiceChangeKeepsCachesAndCapabilitiesBoundToVoice(t *testing.T) {
	input := validSpeechInput()
	bindings := map[string]bool{}
	for _, voice := range []string{rendercontract.DefaultVoice, "en-US-Serena:DragonHDLatestNeural", "en-US-Ava:DragonHDLatestNeural", "en-US-Andrew:DragonHDLatestNeural"} {
		input.Voice = voice
		binding := speechBinding(input, "canadacentral")
		if bindings[binding] {
			t.Fatal("historical voice audio shared a new voice binding")
		}
		bindings[binding] = true
	}
	service := NewService(&fakeSpeechProvider{}, &fakeSpeechLedger{}, "canadacentral")
	service.capabilities = &VoiceCapabilities{Version: "voice-capabilities/1", Revision: "historical-serena/1", Voice: "en-US-Serena:DragonHDLatestNeural", Region: "canadacentral", Locales: map[string]LocaleCapabilities{"en-US": {Available: true, Styles: []string{"curious"}}}}
	reservation := rendercontract.DeliveryReservation{Expressive: true, Version: rendercontract.DeliveryVersion, Token: "fixture_token", Instance: "fixture_instance", Effective: rendercontract.EmotionTarget{State: "curious", Intensity: .3, Energy: .5}, Request: rendercontract.DeliveryRequest{RequestID: "fixture_request", TurnID: "fixture_turn", Revision: 1, TextHash: rendercontract.TextHash("Hello."), Voice: rendercontract.DefaultVoice, Language: "en-US", Purpose: "ask"}}
	if _, err := service.VoiceDirection(reservation); err == nil {
		t.Fatal("Emma inherited Serena capabilities")
	}
	service.capabilities = &VoiceCapabilities{Version: "voice-capabilities/1", Revision: "qualified-emma/1", Voice: rendercontract.DefaultVoice, Region: "canadacentral", Locales: map[string]LocaleCapabilities{"en-US": {Available: true}, "es-CO": {Available: true}}}
	for _, language := range []string{"en-US", "es-CO"} {
		reservation.Request.Language = language
		direction, err := service.VoiceDirection(reservation)
		if err != nil || direction.Style != "" {
			t.Fatal("unqualified Emma style activated", direction, err)
		}
	}
}
