package conversation

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"os"
	"syntelle/render-contract"
	"time"
)

type ExpressionBaseline struct {
	Regulation   rendercontract.EmotionRegulation `json:"emotion_regulation"`
	EmotionPhase string                           `json:"emotion_phase,omitempty"`

	Emotion     rendercontract.EmotionBaseline `json:"emotion_baseline"`
	LastEmotion *rendercontract.EmotionTarget  `json:"last_activated_emotion,omitempty"`
	Smile       float64                        `json:"smile"`
	Brows       float64                        `json:"brows"`
}

func (b ExpressionBaseline) valid() bool {
	return b.Regulation.Valid() && b.Emotion.Valid() && !math.IsNaN(b.Smile) && !math.IsNaN(b.Brows) && b.Smile >= 0 && b.Smile <= 1 && b.Brows >= 0 && b.Brows <= 1
}

type PerformanceProfile struct {
	Voice        string                     `json:"voice,omitempty"`
	MotionPolicy string                     `json:"motion_policy,omitempty"`
	Version      string                     `json:"version"`
	PackageHash  string                     `json:"package_hash"`
	Reviewed     bool                       `json:"reviewed"`
	Capabilities []string                   `json:"capabilities"`
	Laughter     map[string]json.RawMessage `json:"laughter"`
}

func (s *Service) RuntimeVoice() string {
	if s != nil && s.Performance != nil && s.Performance.Voice != "" && rendercontract.SupportedVoice(s.Performance.Voice) {
		return s.Performance.Voice
	}
	return rendercontract.DefaultVoice
}

func (s *Service) HasLaughter(voice, language string) bool {
	if s == nil || s.Performance == nil {
		return false
	}
	_, ok := s.Performance.Laughter[voice+"|"+language]
	return ok
}

func LoadPerformanceProfile(path, packageHash string) (*PerformanceProfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p PerformanceProfile
	if json.Unmarshal(data, &p) != nil || p.Version != "character-performance/1" || !p.Reviewed || p.PackageHash != packageHash || len(packageHash) != 64 {
		return nil, errors.New("invalid reviewed character performance profile")
	}
	if p.Voice != "" && !rendercontract.SupportedVoice(p.Voice) {
		return nil, errors.New("invalid character voice")
	}
	seen := map[string]bool{}
	if p.MotionPolicy != "" && p.MotionPolicy != rendercontract.ConversationalMotionVersion {
		return nil, errors.New("invalid prepared motion policy")
	}
	for _, kind := range p.Capabilities {
		if seen[kind] || (!headCue(kind) && kind != "brows" && kind != "smile" && kind != "laugh") {
			return nil, errors.New("invalid prepared capability")
		}
		seen[kind] = true
	}
	for key, raw := range p.Laughter {
		var cached cachedSpeech
		if json.Unmarshal(raw, &cached) != nil || key != cached.Voice+"|"+cached.Language {
			return nil, errors.New("invalid cached laughter identity")
		}
		format, pcm, err := pcm16(cached.Audio)
		if err != nil {
			return nil, errors.New("invalid cached laughter audio")
		}
		duration := float64(len(pcm)) / float64(binary.LittleEndian.Uint32(format[4:])*2)
		if duration > 2.5 {
			return nil, errors.New("cached laughter exceeds prepared duration")
		}
	}
	return &p, nil
}
func (s *Service) directionContext(b ExpressionBaseline, voiceLanguage ...string) string {
	b.Emotion = b.Emotion.Normalized()
	capabilities := []string{"nod", "shake", "emphasis", "tilt", "brows"}
	if s.Performance != nil {
		capabilities = nil
		for _, kind := range s.Performance.Capabilities {
			if kind != "double_nod" {
				capabilities = append(capabilities, kind)
			}
		}
	}
	if len(voiceLanguage) == 2 {
		kept := make([]string, 0, len(capabilities))
		for _, kind := range capabilities {
			if kind == "laugh" {
				if s.Performance == nil {
					continue
				}
				if _, ok := s.Performance.Laughter[voiceLanguage[0]+"|"+voiceLanguage[1]]; !ok {
					continue
				}
			}
			kept = append(kept, kind)
		}
		capabilities = kept
	}
	value, _ := json.Marshal(map[string]any{"prepared_capabilities": capabilities, "emotion_regulation": map[string]any{"version": rendercontract.EmotionPolicyVersion, "settings": b.Regulation.Normalized(), "temperament": b.Regulation.Profile(), "ceilings": map[string]float64{"joyful": rendercontract.EmotionCeiling("joyful"), "playful": rendercontract.EmotionCeiling("playful"), "affectionate": rendercontract.EmotionCeiling("affectionate"), "curious": rendercontract.EmotionCeiling("curious"), "sad": rendercontract.EmotionCeiling("sad"), "angry": rendercontract.EmotionCeiling("angry"), "fearful": rendercontract.EmotionCeiling("fearful"), "surprised": rendercontract.EmotionCeiling("surprised"), "disgusted": rendercontract.EmotionCeiling("disgusted")}}, "manual_expression_baseline": b, "baseline_policy": "temporary-targets-then-return", "legacy_speaking_smile_maximum": .5, "emotion_policy": "temporary-contextual-targets-then-return-to-owner-baseline"})
	motion := ""
	if s.Performance != nil && s.Performance.MotionPolicy == rendercontract.ConversationalMotionVersion {
		motion = "\nApplication-owned head policy: eight movement cycles per rolling 20 seconds, minimum 2.2-second spacing, semantic priority over acoustic beats. Movement timing continues across phrases; do not treat eight as a whole-reply ceiling."
	}
	return "\nApplication-owned performance context: " + string(value) + motion
}
func (s *Service) filterCapabilities(v PerformanceValidation) PerformanceValidation {
	allowed := map[string]bool{"nod": true, "double_nod": true, "shake": true, "emphasis": true, "tilt": true, "brows": true}
	if s.Performance != nil {
		allowed = map[string]bool{}
		for _, kind := range s.Performance.Capabilities {
			allowed[kind] = true
		}
	}
	kept := make([]ResolvedCue, 0, len(v.Resolved))
	for _, cue := range v.Resolved {
		if allowed[cue.Kind] {
			kept = append(kept, cue)
		} else {
			v.Omitted = append(v.Omitted, "unsupported_"+cue.Kind)
		}
	}
	v.Resolved = kept
	return v
}

type laughterPolicy struct {
	last     time.Time
	previous bool
	reply    string
	blocked  bool
}

func (s *Service) beginPerformance(cid, reply string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.laughPolicy == nil {
		s.laughPolicy = map[string]laughterPolicy{}
	}
	p := s.laughPolicy[cid]
	p.blocked = p.previous
	p.previous = false
	p.reply = reply
	s.laughPolicy[cid] = p
}
func (s *Service) reserveLaughter(cid, reply string) bool {
	return s.laughterAllowed(cid, reply, true)
}
func (s *Service) laughterAllowed(cid, reply string, commit bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.laughPolicy[cid]
	if p.reply != reply || p.blocked || time.Since(p.last) < 60*time.Second || p.previous {
		return false
	}
	if commit {
		p.last = time.Now()
		p.previous = true
		s.laughPolicy[cid] = p
	}
	return true
}
