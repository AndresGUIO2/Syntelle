package speech

import (
	"strings"
	"syntelle/render-contract"
	"testing"
)

func TestEffectiveNegativeEmotionPrecedesPurpose(t *testing.T) {
	for _, state := range []string{"fearful", "sad", "angry"} {
		t.Run(state, func(t *testing.T) {
			s := NewService(&fakeSpeechProvider{}, &fakeSpeechLedger{}, "canadacentral")
			s.capabilities = &VoiceCapabilities{Voice: rendercontract.DefaultVoice, Revision: "emma-qualified-fixture/1", Locales: map[string]LocaleCapabilities{"en-US": {Available: true, Styles: []string{state, "reassuring", "curious", "amused"}}, "es-CO": {Available: true}}}
			r := rendercontract.DeliveryReservation{Expressive: true, Version: rendercontract.DeliveryVersion, Token: "fixture_token", Instance: "fixture_instance", Effective: rendercontract.EmotionTarget{State: state, Intensity: .6, Energy: .5}, Request: rendercontract.DeliveryRequest{RequestID: "fixture_request", TurnID: "fixture_turn", Revision: 1, TextHash: rendercontract.TextHash("Stay with me."), Voice: rendercontract.DefaultVoice, Language: "en-US", Purpose: "reassure"}}
			d, err := s.VoiceDirection(r)
			if err != nil || d.Style != state {
				t.Fatal("purpose overrode approved emotion", d, err)
			}
			i := validSpeechInput()
			i.Voice = rendercontract.DefaultVoice
			directed, err := WithVoiceDelivery(i, d)
			if err != nil || !strings.Contains(directed.ProviderInput(), `style="`+state+`"`) || directed.Text != i.Text {
				t.Fatal("missing exact private serialization", err)
			}
			r.Request.Language = "es-CO"
			d, err = s.VoiceDirection(r)
			if err != nil || d.Style != "" {
				t.Fatal("English qualification transferred to Spanish", d, err)
			}
			r.Request.Language = "en-US"
			s.capabilities.Locales["en-US"] = LocaleCapabilities{Available: true, Styles: []string{"reassuring"}}
			d, err = s.VoiceDirection(r)
			if err != nil || d.Style != "" {
				t.Fatal("unqualified emotion used unrelated reassurance", d, err)
			}
		})
	}
}
