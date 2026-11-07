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

const SemanticMotionDirection = "semantic-anchors/1"

type PerformanceProfile struct {
	MotionDirection   string                            `json:"motion_direction,omitempty"`
	Identity          *CharacterIdentity                `json:"identity,omitempty"`
	EmotionalPresence *rendercontract.EmotionalPresence `json:"emotional_presence,omitempty"`
	Voice             string                            `json:"voice,omitempty"`
	MotionPolicy      string                            `json:"motion_policy,omitempty"`
	Version           string                            `json:"version"`
	PackageHash       string                            `json:"package_hash"`
	Reviewed          bool                              `json:"reviewed"`
	Capabilities      []string                          `json:"capabilities"`
	Laughter          map[string]json.RawMessage        `json:"laughter"`
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
	if err := p.Identity.validate(); err != nil {
		return nil, err
	}
	if p.EmotionalPresence != nil && (p.EmotionalPresence.Version != "emotional-presence/1" || !p.EmotionalPresence.Validation.Valid || p.EmotionalPresence.Minimum != .4 || p.EmotionalPresence.InactivityMS != 180000 || p.EmotionalPresence.RecoveryMS != 60000) {
		return nil, errors.New("invalid emotional presence profile")
	}
	if p.MotionDirection != "" && p.MotionDirection != SemanticMotionDirection {
		return nil, errors.New("invalid motion direction policy")
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
	ceilings := map[string]float64{}
	for _, state := range rendercontract.EmotionStates[1:] {
		ceilings[state] = rendercontract.EmotionCeiling(state)
	}
	regulation := map[string]any{"version": rendercontract.EmotionPolicyVersion, "settings": b.Regulation.Normalized(), "temperament": b.Regulation.Profile(), "ceilings": ceilings}
	policy := "temporary-contextual-targets-then-return-to-owner-baseline"
	if s.Performance != nil && s.Performance.EmotionalPresence != nil {
		for state := range ceilings {
			ceilings[state] = 1
		}
		regulation["minimum_final_automatic_intensity"] = .4
		regulation["inactivity_seconds"], regulation["recovery_seconds"] = 180, 60
		policy = "persistent-committed-mood-across-turns; absolute-contextual-targets; manual-baseline-remains-owner-controlled"
	}
	value, _ := json.Marshal(map[string]any{"prepared_capabilities": capabilities, "emotion_regulation": regulation, "manual_expression_baseline": b, "baseline_policy": policy, "legacy_speaking_smile_maximum": .5, "emotion_policy": policy})
	motion := ""
	if s.Performance != nil && s.Performance.MotionPolicy == rendercontract.ConversationalMotionVersion {
		motion = "\nApplication-owned head policy: eight movement cycles per rolling 20 seconds, minimum 2.2-second spacing, semantic priority over acoustic beats. Movement timing continues across phrases; do not treat eight as a whole-reply ceiling."
	}
	if s.Performance != nil && s.Performance.MotionDirection == SemanticMotionDirection {
		motion += "\nPrepared intentional speaking policy: conspicuous head actions require a meaningful exact semantic anchor; acoustic peaks refine that action, never create one. Keep expressive quick/wide actions for meaningful emphasis; do not limit explanations to one restrained beat and do not fill a quota. Affirm/disagree describe the speaker actually agreeing/disagreeing, not an event narrated in a story. Narrative events such as someone welcoming a traveler do not themselves justify an agreement nod. Use optional tilt/emphasis/brows/smile for communicative purpose, leaving quiet spans. The application retains only confidently timed future actions."
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
