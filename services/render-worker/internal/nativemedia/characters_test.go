package nativemedia

import (
	"encoding/json"
	"errors"
	"strings"
	"syntelle/render-contract"
	"testing"
)

func TestPrivatePackageBindingNeverUsesDefaultFace(t *testing.T) {
	for _, tc := range []struct {
		name, character, hash string
		registry, fail, want  bool
	}{
		{name: "legacy default", character: "syntelle", want: true},
		{name: "private hash", character: "char_" + strings.Repeat("1", 32), hash: strings.Repeat("a", 64), registry: true, want: true},
		{name: "private missing hash", character: "char_" + strings.Repeat("1", 32), registry: true},
		{name: "private missing registry", character: "char_" + strings.Repeat("1", 32), hash: strings.Repeat("a", 64)},
		{name: "private loader fails", character: "char_" + strings.Repeat("1", 32), hash: strings.Repeat("a", 64), registry: true, fail: true},
		{name: "path injection", character: "char_" + strings.Repeat("1", 32), hash: "../assets", registry: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			factory := func(rendercontract.Settings) (Scene, error) { return nil, nil }
			h := &Host{config: Config{PackageHash: "default", SceneFactory: factory}}
			if tc.registry {
				h.config.ResolvePackage = func(hash string) (SceneFactory, error) {
					if hash != tc.hash {
						t.Fatal(hash)
					}
					if tc.fail {
						return nil, errors.New("unavailable")
					}
					return factory, nil
				}
			}
			settings := rendercontract.Defaults()
			settings.Character = tc.character
			_, hash, err := h.resolveScene(settings, tc.hash)
			if (err == nil) != tc.want {
				t.Fatalf("%s %v", hash, err)
			}
			if tc.character != "syntelle" && err == nil && hash != tc.hash {
				t.Fatal("default face substituted")
			}
		})
	}
}

func TestBasicCharacterPreflightKeepsReservationAndNeutralVoice(t *testing.T) {
	s, h, messages := deliveryFixture(t)
	s.scene = nil // no emotional resource, as in the standard portrait package
	reserveFixture(t, s, h, 0)
	var notice rendercontract.DeliveryNotice
	if err := json.Unmarshal((*messages)[0].Data, &notice); err != nil {
		t.Fatal(err)
	}
	if notice.Status != "reserved" || !notice.Reservation.Valid() || notice.Reservation.Expressive || notice.Reservation.Effective.State != "neutral" || notice.Reservation.Effective.Intensity != 0 {
		t.Fatal(notice)
	}
	if !s.settings.AutomaticExpressions || !s.settings.Movement || s.emotionController.Sequence != 0 {
		t.Fatal("preflight changed presentation")
	}
}

func TestConfigureCannotSwitchPreparedCharacter(t *testing.T) {
	s := queuedSession()
	next := s.settings
	next.Character = "char_" + strings.Repeat("1", 32)
	if err := s.configure(conversationData(t, next)); err == nil {
		t.Fatal("active package switched")
	}
	if s.settings.Character != rendercontract.Defaults().Character {
		t.Fatal("changed active character")
	}
}
