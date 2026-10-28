//go:build integration

package conversation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestExportCompleteLaughterProductionFixtures(t *testing.T) {
	folder := os.Getenv("SYNTELLE_LAUGHTER_REVIEW")
	if folder == "" {
		t.Skip("explicit cached review only")
	}
	profileBytes, err := os.ReadFile(filepath.Join(folder, "../jaw-articulation-108/profile/profile.json"))
	if err != nil {
		t.Fatal(err)
	}
	var profile PerformanceProfile
	if err = json.Unmarshal(profileBytes, &profile); err != nil {
		t.Fatal(err)
	}
	var outputs []map[string]any
	for _, language := range []string{"en-US", "es-CO"} {
		data, err := os.ReadFile(filepath.Join(folder, "../emma-presence-107/replay", language+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var replays []struct {
			Phrase PerformanceIntent `json:"phrase"`
			Speech json.RawMessage   `json:"speech"`
		}
		if err = json.Unmarshal(data, &replays); err != nil {
			t.Fatal(err)
		}
		service := &Service{Performance: &profile}
		preview, err := service.LaughterPreview(profile.Voice, language)
		if err != nil {
			t.Fatal(err)
		}
		outputs = append(outputs, map[string]any{"name": language + "-manual", "text": "Cached laughter preview", "speech": preview, "manual": true})
		for _, amount := range []float64{.2, .5, 1} {
			service = &Service{Performance: &profile}
			service.beginPerformance("review", "reply")
			var speech map[string]any
			json.Unmarshal(replays[0].Speech, &speech)
			intent := PerformanceIntent{Version: "performance-intent/4", Purpose: "playful", Text: replays[0].Phrase.Text, Tone: "playful", Cues: []PerformanceCue{{Kind: "laugh", Anchor: replays[0].Phrase.Text, Amount: &amount}}}
			direction := PerformanceValidation{Intent: intent, Resolved: []ResolvedCue{{Kind: "laugh", Time: 0, Duration: 1, Amount: &amount}}}
			effective, err := service.appendLaughter("review", "reply", profile.Voice, language, speech, direction)
			if err != nil {
				t.Fatal(err)
			}
			if len(effective.Resolved) != 1 || *effective.Resolved[0].Amount != 1 {
				t.Fatal("complete action not admitted", effective)
			}
			speech["performance"] = effective
			outputs = append(outputs, map[string]any{"name": language + "-automatic-" + strconv.Itoa(int(amount*10)), "text": intent.Text, "speech": speech, "requested_amount": amount, "manual": false})
		}
	}
	data, _ := json.Marshal(outputs)
	if err = os.WriteFile(filepath.Join(folder, "production-inputs.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
}
