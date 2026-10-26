package conversation

import (
	"encoding/json"
	"syntelle/render-contract"
	"testing"
)

func TestV3PhrasesKeepSpeechWhenOptionalEmotionIsInvalid(t *testing.T) {
	for _, emotion := range []string{`{"state":"sad","intensity":0.7,"energy":0.3}`, `{"state":"invalid","intensity":0.7,"energy":0.3}`, `{"state":"sad","intensity":0.7}`, `{"state":"sad","intensity":0.7,"energy":0.3,"geometry":"forbidden"}`} {
		raw := `{"version":"performance-intent/3","tone":"neutral","phrases":[{"text":"I understand.","emotion":` + emotion + `,"cues":[]}]}`
		p, err := completePhrases(raw)
		if err != nil || len(p) != 1 || p[0].Text != "I understand." || p[0].Version != "performance-intent/3" {
			t.Fatal(p, err)
		}
		expected := emotion == `{"state":"sad","intensity":0.7,"energy":0.3}`
		if (p[0].Emotion != nil) != expected {
			t.Fatal("incorrect optional emotion admission", emotion)
		}
		partial, err := decodedPhrases(raw[:len(raw)-2])
		if err != nil || len(partial) != 1 {
			t.Fatal("complete phrase not streamed", partial, err)
		}
	}
}

func TestBaselineDoesNotAcceptCallerSuppliedContext(t *testing.T) {
	last := &rendercontract.EmotionTarget{State: "sad", Intensity: .6, Energy: .2}
	before := ExpressionBaseline{Emotion: rendercontract.EmotionBaseline{State: "joyful", Intensity: .4}, LastEmotion: last}
	next := before
	next.LastEmotion = &rendercontract.EmotionTarget{State: "angry", Intensity: 1, Energy: 1}
	if updateExpressionBaseline(before, next).LastEmotion != last {
		t.Fatal("caller overwrote actual activated state")
	}
	next.Emotion.State = "curious"
	if updateExpressionBaseline(before, next).LastEmotion != nil {
		t.Fatal("old contextual state survived a baseline edit")
	}
}

func TestBrowserCannotSupplyPrivateEmotionActivation(t *testing.T) {
	var event Event
	if err := json.Unmarshal([]byte(`{"type":"emotion_activated","emotion_activation":{"turn_id":"turn_fixture","emotion":{"state":"angry","intensity":1,"energy":1}}}`), &event); err != nil {
		t.Fatal(err)
	}
	if event.EmotionActivation != nil || event.EmotionDecision != nil {
		t.Fatal("browser fabricated private playback context")
	}
}

func TestBrowserCannotSupplyPolicyDecisions(t *testing.T) {
	var event Event
	if err := json.Unmarshal([]byte(`{"type":"emotion_decision","emotion_decision":{"effective":{"state":"angry","intensity":1,"energy":1}}}`), &event); err != nil {
		t.Fatal(err)
	}
	if event.EmotionDecision != nil {
		t.Fatal("browser fabricated effective policy context")
	}
}
