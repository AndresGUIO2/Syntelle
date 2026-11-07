package streaming

import (
	"context"
	"errors"
	"strings"
	"syntelle/api/internal/accounts"
	"testing"
)

func TestResolvePackageRequiresReadyOwnership(t *testing.T) {
	id := "char_" + strings.Repeat("1", 32)
	for _, tc := range []struct {
		name, kind, owner, character, resolved string
		denied                                 bool
		want                                   bool
	}{
		{name: "default", character: "syntelle", want: true},
		{name: "owned ready", kind: "account", owner: "owner", character: id, resolved: strings.Repeat("a", 64), want: true},
		{name: "guest", kind: "guest", character: id},
		{name: "different owner", kind: "account", owner: "other", character: id, denied: true},
		{name: "not ready", kind: "account", owner: "owner", character: id, denied: true},
		{name: "untrusted path", kind: "account", owner: "owner", character: "../private"},
		{name: "invalid registry hash", kind: "account", owner: "owner", character: id, resolved: "../private"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			service := &Service{config: Config{PackageHash: "default", ResolveCharacter: func(ctx context.Context, owner, character string) (string, error) {
				calls++
				if owner != tc.owner || character != id {
					t.Fatal("identity changed")
				}
				if tc.denied {
					return "", errors.New("missing")
				}
				return tc.resolved, nil
			}}}
			hash, err := service.resolvePackage(context.Background(), accounts.Principal{Kind: tc.kind, AccountID: tc.owner}, tc.character)
			if (err == nil) != tc.want {
				t.Fatalf("hash %q error %v", hash, err)
			}
			if (tc.kind != "account" || tc.character == "syntelle" || tc.character == "../private") && calls != 0 {
				t.Fatal("unauthorized resolver invocation")
			}
		})
	}
}

func TestBasicCharacterCapabilitiesDoNotAlterArticulationControls(t *testing.T) {
	settings := accounts.Defaults()
	settings.Smile = .3
	settings.Brows = .2
	normalizeBasicSettings(&settings)
	if settings.EmotionBaseline.State != "neutral" || settings.EmotionBaseline.Intensity != 0 || !settings.Movement || !settings.Blinking || !settings.AutomaticExpressions || settings.Smile != .3 || settings.Brows != .2 {
		t.Fatal(settings)
	}
}
