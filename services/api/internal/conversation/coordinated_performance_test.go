package conversation

import "testing"

func TestCoordinatedPhrasePurpose(t *testing.T) {
	valid := `{"version":"performance-intent/4","tone":"serious","phrases":[{"text":"We can take this one step at a time.","purpose":"reassure","emotion":{"state":"affectionate","intensity":0.3,"energy":0.4},"cues":[]}]}`
	phrases, err := completePhrases(valid)
	if err != nil || len(phrases) != 1 || phrases[0].Version != "performance-intent/4" {
		t.Fatalf("coordinated direction rejected: %v %v", phrases, err)
	}
	invalid := `{"version":"performance-intent/4","tone":"neutral","phrases":[{"text":"Hello.","purpose":"execute","cues":[]}]}`
	if _, err := completePhrases(invalid); err == nil {
		t.Fatal("unknown communicative purpose admitted")
	}
}
