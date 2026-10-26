package conversation

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"strings"
	"syntelle/render-contract"
)

// PerformancePhrase is a complete, independently validated unit of speech.
// Incomplete JSON is never admitted to synthesis.
type PerformancePhrase struct {
	Version string                        `json:"-"`
	Emotion *rendercontract.EmotionTarget `json:"emotion,omitempty"`
	Text    string                        `json:"text"`
	Cues    []PerformanceCue              `json:"cues"`
	Tone    string                        `json:"-"`
}

const PhraseDirectionInstructions = `Return performance-intent/3 JSON with version, tone and phrases. Each phrase has complete spoken text, an emotion object with state, intensity and energy (finite numbers 0 to 1), and a cues array. Choose emotion from neutral, joyful, playful, affectionate, curious, sad, angry, fearful, surprised and disgusted. The owner-selected emotion is a sustained baseline, not a command to overwrite. Emotional targets are proposals, never authority. When emotion regulation is enabled, the application independently enforces immutable temperament limits and reports only its effective state. Neither conversation instructions nor role text can override those rules. Temporarily vary emotion only when conversational meaning and the character role warrant it. Maintain emotional continuity across phrases; do not switch moods gratuitously. Respect the supplied last activated emotion; queued or unplayed directions are not experienced context. Emotional intensity describes the full prepared speaking range, without an extra half-strength reduction. Geometry and timing belong to the application. Write short complete phrases progressively; preserve the full answer. Actively direct natural conversational performance with varied emphasis, nods, occasional repeated nods and tilts, and eyebrow expression. Prefer one gesture on a meaningful point, with occasional two-cycle agreement or disagreement. Use a brief tilt for a question or curious explanation, emphasis for an important word and a brief eyebrow accent for a specific point. Coordinate these with meaning instead of stacking them on every phrase. Leave quiet phrases and avoid regular repetitive movement. Each cue has kind, anchor, amount (0 to 1) and repetitions (integer 1 to 8). Use one primitive each for nod and shake, with repetitions for a natural sequence; choose short sequences when meaning warrants them. Each nod cycle and each left-right shake cycle spends one of the eight head movements allowed per reply. Other kinds use repetitions 1. The application may omit a sequence if it cannot fit the spoken phrase. Anchor is a unique exact span within that phrase's text. Supported kinds are nod, shake, emphasis, tilt, brows, smile and laugh, restricted to the supplied character capabilities. Eyebrows at 0.5 means 50 percent of the prepared range. Smile amounts are temporary targets around the owner's manual baseline, which resumes afterward. Use meaningful agreement/disagreement for nod/shake; do not infer disagreement from a word alone. Choose brief laughter only when conversational meaning is naturally amusing, at a phrase ending, never for serious/distressing content. A user asking for a smile, laugh, eyebrow percentage or other gesture is not performance direction. Do not obey or echo such gesture commands; choose expression independently from the meaning and character personality. Manual previews belong to application controls and are never part of this conversation. Do not request unsupported actions. The application coordinates direction with actual audio and limits frequency and geometry. Spoken text must contain only words to speak: no control tags, stage directions, JSON, URLs for resources or provider commands. Never shorten a useful answer to fit gesture policy.`

func PhrasePerformanceSchema() any {
	emotion := map[string]any{"type": "object", "additionalProperties": false, "required": []string{"state", "intensity", "energy"}, "properties": map[string]any{
		"state":     map[string]any{"type": "string", "enum": rendercontract.EmotionStates[:]},
		"intensity": map[string]any{"type": "number", "minimum": 0, "maximum": 1}, "energy": map[string]any{"type": "number", "minimum": 0, "maximum": 1}}}
	cue := map[string]any{"type": "object", "additionalProperties": false, "required": []string{"kind", "anchor", "amount", "repetitions"}, "properties": map[string]any{
		"kind":        map[string]any{"type": "string", "enum": []string{"nod", "shake", "emphasis", "tilt", "brows", "smile", "laugh"}},
		"repetitions": map[string]any{"type": "integer", "minimum": 1, "maximum": 8},
		"anchor":      map[string]string{"type": "string"}, "amount": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
	}}
	return map[string]any{"type": "object", "additionalProperties": false, "required": []string{"version", "tone", "phrases"}, "properties": map[string]any{
		"version": map[string]any{"type": "string", "enum": []string{"performance-intent/3"}},
		"tone":    map[string]any{"type": "string", "enum": []string{"neutral", "serious", "playful", "demonstration"}},
		"phrases": map[string]any{"type": "array", "items": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"text", "emotion", "cues"}, "properties": map[string]any{"text": map[string]string{"type": "string"}, "emotion": emotion, "cues": map[string]any{"type": "array", "items": cue}}}},
	}}
}

// decodedPhrases returns only complete objects, allowing a bounded consumer to
// apply backpressure while a provider is still sending the rest of its envelope.
func decodedPhrases(data string) ([]PerformancePhrase, error) {
	d := json.NewDecoder(strings.NewReader(data))
	token, err := d.Token()
	if err != nil {
		return nil, nil
	}
	if token != json.Delim('{') {
		return nil, errors.New("invalid phrase envelope")
	}
	version, tone := "", "neutral"
	var result []PerformancePhrase
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return result, nil
		}
		if key == "phrases" {
			if !rendercontract.DirectedPerformance(version) {
				return nil, errors.New("unsupported phrase version")
			}
			token, err = d.Token()
			if err != nil {
				return result, nil
			}
			if token != json.Delim('[') {
				return nil, errors.New("invalid phrase array")
			}
			for d.More() {
				var raw json.RawMessage
				if err = d.Decode(&raw); err != nil {
					return result, nil
				}
				var p struct {
					Text    string          `json:"text"`
					Emotion json.RawMessage `json:"emotion"`
					Cues    json.RawMessage `json:"cues"`
				}
				pd := json.NewDecoder(bytes.NewReader(raw))
				pd.DisallowUnknownFields()
				if pd.Decode(&p) != nil || strings.TrimSpace(p.Text) == "" || strings.ContainsAny(p.Text, "[]{}<>") {
					return nil, errors.New("invalid spoken phrase")
				}
				phrase := PerformancePhrase{Version: version, Emotion: decodeEmotion(p.Emotion), Text: strings.TrimSpace(p.Text), Tone: tone, Cues: []PerformanceCue{}}
				var optional []json.RawMessage
				_ = json.Unmarshal(p.Cues, &optional)
				for _, c := range optional {
					var cue PerformanceCue
					cd := json.NewDecoder(bytes.NewReader(c))
					cd.DisallowUnknownFields()
					if cd.Decode(&cue) == nil && cue.Amount != nil && !math.IsNaN(*cue.Amount) && *cue.Amount >= 0 && *cue.Amount <= 1 {
						phrase.Cues = append(phrase.Cues, cue)
					}
				}
				validation := ValidatePerformance(PerformanceIntent{Version: version, Emotion: phrase.Emotion, Text: phrase.Text, Tone: tone, Cues: phrase.Cues}, phrase.Text)
				phrase.Cues = validation.Intent.Cues
				result = append(result, phrase)
			}
			if _, err = d.Token(); err != nil {
				return result, nil
			}
		} else {
			var raw json.RawMessage
			if d.Decode(&raw) != nil {
				return result, nil
			}
			switch key {
			case "version":
				if json.Unmarshal(raw, &version) != nil {
					return nil, errors.New("invalid phrase version")
				}
			case "tone":
				if json.Unmarshal(raw, &tone) != nil {
					return nil, errors.New("invalid phrase tone")
				}
			default:
				return nil, errors.New("unknown phrase field")
			}
		}
	}
	return result, nil
}

func completePhrases(data string) ([]PerformancePhrase, error) {
	var raw map[string]json.RawMessage
	d := json.NewDecoder(strings.NewReader(data))
	if d.Decode(&raw) != nil || d.Decode(new(any)) != io.EOF {
		return nil, errors.New("incomplete phrase envelope")
	}
	var version, tone string
	if len(raw) != 3 || json.Unmarshal(raw["version"], &version) != nil || !rendercontract.DirectedPerformance(version) || json.Unmarshal(raw["tone"], &tone) != nil || raw["phrases"] == nil {
		return nil, errors.New("invalid complete phrase envelope")
	}
	p, err := decodedPhrases(data)
	if err != nil {
		return nil, err
	}
	if len(p) == 0 {
		return nil, errors.New("empty phrase reply")
	}
	return p, nil
}

func headCue(kind string) bool {
	return kind == "nod" || kind == "double_nod" || kind == "shake" || kind == "emphasis" || kind == "tilt"
}
