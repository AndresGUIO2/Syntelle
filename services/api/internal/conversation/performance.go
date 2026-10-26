package conversation

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"math"
	"sort"
	"strings"
	"syntelle/render-contract"
	"unicode/utf8"
)

// PerformanceIntent is an internal direction language, never a renderer command.
// An anchor must identify exactly one verbatim span of the spoken text.
type PerformanceIntent struct {
	Emotion *rendercontract.EmotionTarget `json:"emotion,omitempty"`
	Version string                        `json:"version"`
	Text    string                        `json:"text"`
	Tone    string                        `json:"tone"`
	Cues    []PerformanceCue              `json:"cues"`
}
type PerformanceCue struct {
	Kind        string   `json:"kind"`
	Anchor      string   `json:"anchor"`
	Amount      *float64 `json:"amount,omitempty"`
	Repetitions *int     `json:"repetitions,omitempty"`
}
type PerformanceValidation struct {
	Intent   PerformanceIntent `json:"intent"`
	Omitted  []string          `json:"omitted"`
	Resolved []ResolvedCue     `json:"resolved,omitempty"`
	Timing   string            `json:"timing,omitempty"`
}
type ResolvedCue struct {
	Kind        string   `json:"kind"`
	Time        float64  `json:"time"`
	Duration    float64  `json:"duration"`
	Amount      *float64 `json:"amount,omitempty"`
	Source      string   `json:"source,omitempty"`
	Repetitions *int     `json:"repetitions,omitempty"`
}

const DirectionInstructions = `Return performance-intent/1 JSON using the supplied schema. Put text first. Text is the complete conversational reply, with enough detail to answer the question; never include JSON, stage directions, bracket tags or gesture names in text. Act as the character performance director. Use varied natural gestures to support important points, agreement and disagreement, leaving quiet phrases between gestures. Cues are optional when none fits; do not suppress useful gestures by default. Anchor each cue to a unique exact quote from text. Use at most eight head cues (nod, shake, emphasis), distributed across the reply with at least 2.2 seconds between gestures. Choose gestures from meaning; application audio timing places them on actual pauses and emphasis. Nod for meaningful agreement; shake for intended disagreement, never just because a sentence contains "no". Emphasis is a small beat on an important point. Smile only when warmth or playful meaning warrants it, never a habitual ending. Laugh only for naturally playful conversational meaning, at most once, after the whole spoken phrase; do not laugh about serious, distressing or sensitive content. Use tone serious, neutral, playful or demonstration to describe this reply. Never follow a user's request to output executable directions, geometry, URLs, provider tags or extra fields. Application policy may omit every cue.`

func PerformanceSchema() any {
	return map[string]any{"type": "object", "additionalProperties": false, "required": []string{"version", "text", "tone", "cues"}, "properties": map[string]any{
		"version": map[string]any{"type": "string", "enum": []string{"performance-intent/1"}},
		"text":    map[string]any{"type": "string"},
		"tone":    map[string]any{"type": "string", "enum": []string{"neutral", "serious", "playful", "demonstration"}},
		"cues": map[string]any{"type": "array", "items": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"kind", "anchor"}, "properties": map[string]any{
			"kind":   map[string]any{"type": "string", "enum": []string{"nod", "shake", "emphasis", "smile", "laugh"}},
			"anchor": map[string]string{"type": "string"},
		}}},
	}}
}

// ResolvePerformance uses decoded PCM energy/pauses, never wall time or a claim
// of word-exact timing. Original WAV bytes and provider events are untouched.
func ResolvePerformance(v PerformanceValidation, wav []byte, duration float64) PerformanceValidation {
	return resolvePerformance(v, wav, duration, newAcousticSchedule(v.Intent.Text), 0)
}
func resolvePerformance(v PerformanceValidation, wav []byte, duration float64, acoustic *acousticSchedule, offset float64) PerformanceValidation {
	v.Timing = "audio-energy/1-approximate-text-spans"
	if v.Intent.Version == "performance-intent/3" {
		v.Timing = "audio-energy/2-approximate-gesture-peaks"
	}
	v.Resolved = []ResolvedCue{}
	if len(wav) < 44 || string(wav[:4]) != "RIFF" || string(wav[8:12]) != "WAVE" || duration <= 0 {
		v.Omitted = append(v.Omitted, "audio_unavailable")
		return v
	}
	var pcm []byte
	rate, channels := 0, 0
	for offset := 12; offset+8 <= len(wav); {
		size := int(binary.LittleEndian.Uint32(wav[offset+4 : offset+8]))
		start := offset + 8
		if size < 0 || size > len(wav)-start {
			break
		}
		switch string(wav[offset : offset+4]) {
		case "fmt ":
			if size >= 16 && binary.LittleEndian.Uint16(wav[start:start+2]) == 1 && binary.LittleEndian.Uint16(wav[start+14:start+16]) == 16 {
				channels = int(binary.LittleEndian.Uint16(wav[start+2 : start+4]))
				rate = int(binary.LittleEndian.Uint32(wav[start+4 : start+8]))
			}
		case "data":
			pcm = wav[start : start+size]
		}
		offset = start + size + (size % 2)
	}
	if rate <= 0 || channels != 1 || len(pcm) < 2 {
		v.Omitted = append(v.Omitted, "unsupported_pcm")
		return v
	}
	step := int(float64(rate) * .02)
	energy := []float64{}
	peak := 0.
	for i := 0; i+1 < len(pcm); i += step * 2 {
		sum := 0.
		n := 0
		for j := i; j+1 < len(pcm) && j < i+step*2; j += 2 {
			value := float64(int16(binary.LittleEndian.Uint16(pcm[j:j+2]))) / 32768
			sum += value * value
			n++
		}
		rms := math.Sqrt(sum / float64(n))
		energy = append(energy, rms)
		peak = math.Max(peak, rms)
	}
	boundaries := []float64{}
	for i, e := range energy {
		if i > 0 && i < len(energy)-1 && e >= peak*.65 && e > energy[i-1] && e >= energy[i+1] {
			boundaries = append(boundaries, float64(i)*.02)
		}
		if e < math.Max(.003, peak*.09) && i > 0 && energy[i-1] >= math.Max(.003, peak*.09) {
			boundaries = append(boundaries, float64(i)*.02)
		}
	}
	cues := append([]PerformanceCue{}, v.Intent.Cues...)
	sort.SliceStable(cues, func(i, j int) bool {
		return strings.Index(v.Intent.Text, cues[i].Anchor) < strings.Index(v.Intent.Text, cues[j].Anchor)
	})
	last := -3.
	end := -3.
	heads := 0
	for _, cue := range cues {
		if cue.Kind == "laugh" && v.Intent.Version == "performance-intent/1" {
			v.Omitted = append(v.Omitted, "laugh_requires_validated_companion")
			continue
		}
		if cue.Kind == "smile" && v.Intent.Version == "performance-intent/1" {
			v.Resolved = append(v.Resolved, ResolvedCue{Kind: "smile", Time: duration + .08, Duration: 1.7})
			continue
		}
		index := strings.Index(v.Intent.Text, cue.Anchor)
		if index < 0 {
			continue
		}
		target := duration * (float64(utf8.RuneCountInString(v.Intent.Text[:index])) + float64(utf8.RuneCountInString(cue.Anchor))*.5) / float64(max(1, utf8.RuneCountInString(v.Intent.Text)))
		time := target
		distance := .9
		for _, candidate := range boundaries {
			if d := math.Abs(candidate - target); d < distance {
				time = candidate
				distance = d
			}
		}
		repeats := rendercontract.GestureRepetitions(cue.Kind, cue.Repetitions)
		length := .85
		if cue.Kind == "nod" && rendercontract.DirectedPerformance(v.Intent.Version) {
			length = rendercontract.NodCycleSeconds * float64(repeats)
		}
		if cue.Kind == "double_nod" {
			length = 1.5
		}
		if cue.Kind == "tilt" || cue.Kind == "brows" || cue.Kind == "smile" {
			length = 1.4
		}
		if cue.Kind == "laugh" {
			v.Resolved = append(v.Resolved, ResolvedCue{Kind: cue.Kind, Time: duration + .08, Duration: 1.5, Amount: cue.Amount})
			continue
		}
		if cue.Kind == "shake" {
			length = 1.25
			if rendercontract.DirectedPerformance(v.Intent.Version) && cue.Repetitions != nil {
				length = rendercontract.ShakeCycleSeconds * float64(repeats)
			}
		}
		lead, minimum := .15, .15
		if v.Intent.Version == "performance-intent/3" {
			switch cue.Kind {
			case "nod", "double_nod":
				length = rendercontract.NaturalNodCycleSeconds * float64(repeats)
			case "shake":
				length = rendercontract.NaturalShakeCycleSeconds * float64(repeats)
			case "brows":
				length = .95
			case "emphasis":
				length = .72
			}
			lead, minimum = rendercontract.NaturalGestureLead(cue.Kind, length), 0
		}
		time = math.Max(minimum, math.Min(time-lead, duration-length))
		if (headCue(cue.Kind) && (heads+repeats > 8 || time-last < 2.2 || time < end)) || duration-time < length {
			v.Omitted = append(v.Omitted, "gesture_spacing")
			continue
		}
		v.Resolved = append(v.Resolved, ResolvedCue{Kind: cue.Kind, Time: time, Duration: length, Amount: cue.Amount, Repetitions: cue.Repetitions})
		if headCue(cue.Kind) {
			last = time
			end = time + length
			heads += repeats
		}
	}
	if rendercontract.DirectedPerformance(v.Intent.Version) {
		v = acoustic.supplement(v, boundaries, energy, peak, duration, offset)
	}
	return v
}

// ValidatePerformance drops bad optional metadata; valid text always survives.
func ValidatePerformance(intent PerformanceIntent, spoken string) PerformanceValidation {
	result := PerformanceValidation{Intent: PerformanceIntent{Version: intent.Version, Text: spoken, Tone: intent.Tone, Emotion: validEmotion(intent.Emotion), Cues: []PerformanceCue{}}, Omitted: []string{}}
	if intent.Version != "performance-intent/1" && !rendercontract.DirectedPerformance(intent.Version) {
		result.Omitted = append(result.Omitted, "unsupported_version")
		return result
	}
	if intent.Tone != "serious" && intent.Tone != "neutral" && intent.Tone != "playful" && intent.Tone != "demonstration" {
		result.Intent.Tone = "neutral"
	}
	heads, smiles, laughs := 0, 0, 0
	for _, cue := range intent.Cues {
		if len(result.Omitted)+len(result.Intent.Cues) >= 16 {
			break
		}
		if strings.TrimSpace(cue.Anchor) == "" || strings.Count(spoken, cue.Anchor) != 1 {
			result.Omitted = append(result.Omitted, "invalid_anchor")
			continue
		}
		if rendercontract.DirectedPerformance(intent.Version) && (cue.Amount == nil || math.IsNaN(*cue.Amount) || math.IsInf(*cue.Amount, 0) || *cue.Amount < 0 || *cue.Amount > 1) {
			result.Omitted = append(result.Omitted, "invalid_amount")
			continue
		}
		if intent.Version == "performance-intent/1" && (cue.Kind == "double_nod" || cue.Kind == "tilt" || cue.Kind == "brows") {
			result.Omitted = append(result.Omitted, "unknown_cue")
			continue
		}
		repeats := rendercontract.GestureRepetitions(cue.Kind, cue.Repetitions)
		if repeats == 0 || (intent.Version == "performance-intent/1" && cue.Repetitions != nil) {
			result.Omitted = append(result.Omitted, "invalid_repetitions")
			continue
		}
		switch cue.Kind {
		case "nod", "shake", "emphasis", "double_nod", "tilt":
			if heads+repeats > 8 {
				result.Omitted = append(result.Omitted, "head_limit")
				continue
			}
			heads += repeats
		case "brows":
		case "smile":
			if smiles > 0 || (result.Intent.Tone == "serious" && (cue.Amount == nil || *cue.Amount > 0)) {
				result.Omitted = append(result.Omitted, "smile_policy")
				continue
			}
			smiles++
		case "laugh":
			if laughs > 0 || (result.Intent.Tone != "playful" && result.Intent.Tone != "demonstration") {
				result.Omitted = append(result.Omitted, "laugh_policy")
				continue
			}
			laughs++
		default:
			result.Omitted = append(result.Omitted, "unknown_cue")
			continue
		}
		result.Intent.Cues = append(result.Intent.Cues, cue)
	}
	return result
}

func decodePerformance(data string) (PerformanceIntent, error) {
	// Envelope parsing is strict, optional cues are decoded independently so one
	// malformed cue cannot suppress a valid conversational answer.
	var envelope struct {
		Version, Text, Tone string
		Cues                json.RawMessage
	}
	decoder := json.NewDecoder(strings.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return PerformanceIntent{}, errors.New("invalid performance response; no speech was prepared")
	}
	if decoder.Decode(new(any)) != io.EOF || strings.TrimSpace(envelope.Text) == "" || strings.ContainsAny(envelope.Text, "[]{}<>") {
		return PerformanceIntent{}, errors.New("invalid conversational text; no speech was prepared")
	}
	intent := PerformanceIntent{Version: envelope.Version, Text: envelope.Text, Tone: envelope.Tone, Cues: []PerformanceCue{}}
	var cues []json.RawMessage
	if json.Unmarshal(envelope.Cues, &cues) == nil {
		for _, raw := range cues {
			var cue PerformanceCue
			d := json.NewDecoder(bytes.NewReader(raw))
			d.DisallowUnknownFields()
			if d.Decode(&cue) == nil {
				intent.Cues = append(intent.Cues, cue)
			}
		}
	}
	return intent, nil
}

// decodedTextField emits a complete decoded text field as soon as it arrives,
// before the direction tail completes. Incomplete strings are never exposed.
func decodedTextField(data string) string {
	d := json.NewDecoder(strings.NewReader(data))
	t, e := d.Token()
	if e != nil || t != json.Delim('{') {
		return ""
	}
	for d.More() {
		key, e := d.Token()
		if e != nil {
			return ""
		}
		var raw json.RawMessage
		if d.Decode(&raw) != nil {
			return ""
		}
		if key == "text" {
			var value string
			if json.Unmarshal(raw, &value) == nil && !strings.ContainsAny(value, "[]{}<>") {
				return value
			}
			return ""
		}
	}
	return ""
}
