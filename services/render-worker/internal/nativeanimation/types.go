// Package nativeanimation samples the existing private character behavior without
// a browser. Each Engine has one owner and must be sampled sequentially on the
// caller's pause-aware animation clock; speech position comes from PCM samples.
package nativeanimation

import (
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"syntelle/render-contract"
)

type Head struct {
	Yaw   float64 `json:"yaw"`
	Pitch float64 `json:"pitch"`
	Roll  float64 `json:"roll"`
}
type Mouth struct {
	From   string  `json:"from"`
	To     string  `json:"to"`
	Amount float64 `json:"amount"`
}
type Frame struct {
	LaughSpeech                     bool `json:"laugh_speech,omitempty"`
	JawOpen, LipOpening, LipTension float64
	Emotion                         rendercontract.EmotionWeights `json:"emotion,omitempty"`
	Head                            Head                          `json:"head"`
	Breath                          float64                       `json:"breath"`
	Mouth                           Mouth                         `json:"mouth"`
	EyeClosure                      float64                       `json:"eye_closure"`
	Gaze                            [2][2]float64                 `json:"gaze"`
	Smile                           float64                       `json:"smile"`
	Brows                           float64                       `json:"brows"`
	Laugh                           float64                       `json:"laugh,omitempty"`
}
type Settings struct {
	ManualLaughter bool             `json:"-"`
	Activity       string           `json:"activity,omitempty"`
	EmotionControl *EmotionSnapshot `json:"-"`
	EmotionTime    float64          `json:"-"`

	EmotionBaseline      rendercontract.EmotionBaseline `json:"emotion_baseline"`
	Movement             bool                           `json:"movement"`
	Blinking             bool                           `json:"blinking"`
	AutomaticExpressions bool                           `json:"automatic_expressions"`
	Smile                float64                        `json:"smile"`
	Brows                float64                        `json:"brows"`
	ReducedMotion        bool                           `json:"reduced_motion"`
	Paused               bool                           `json:"paused"`
}

func DefaultSettings() Settings {
	return Settings{Movement: true, Blinking: true, AutomaticExpressions: true}
}

type Bounds struct {
	Yaw   float64 `json:"yaw"`
	Pitch float64 `json:"pitch"`
	Roll  float64 `json:"roll"`
}
type BlinkSettings struct {
	Closing         float64 `json:"closing_ms"`
	Closed          float64 `json:"closed_ms"`
	Opening         float64 `json:"opening_ms"`
	MinimumInterval float64 `json:"minimum_interval_ms"`
	MaximumInterval float64 `json:"maximum_interval_ms"`
}
type DoubleSettings struct {
	Probability     float64    `json:"probability"`
	OpenGap         [2]float64 `json:"open_gap_ms"`
	MinimumInterval float64    `json:"minimum_interval_ms"`
}
type BehaviorBlink struct {
	Rate       float64         `json:"rate_per_minute"`
	MinimumGap float64         `json:"minimum_gap_ms"`
	Pause      float64         `json:"pause_probability"`
	Ending     float64         `json:"ending_probability"`
	Onset      float64         `json:"onset_probability"`
	Nod        float64         `json:"nod_probability"`
	Double     *DoubleSettings `json:"double"`
}
type BehaviorHead struct {
	Idle              []float64 `json:"idle_degrees"`
	MaximumNod        float64   `json:"maximum_nod_degrees"`
	MaximumTurn       float64   `json:"maximum_turn_degrees"`
	MinimumGestureGap float64   `json:"minimum_gesture_gap_seconds"`
}
type Behavior struct {
	Head  BehaviorHead  `json:"head"`
	Blink BehaviorBlink `json:"blink"`
}
type Breathing struct {
	Duration  [2]float64 `json:"duration_ms"`
	Inhale    [2]float64 `json:"inhale_fraction"`
	Amplitude [2]float64 `json:"amplitude"`
}
type GazeEye struct {
	Width  float64       `json:"width_px"`
	Bounds [2][2]float64 `json:"bounds_px"`
}
type Config struct {
	LaughterV2           bool
	Mandible             *rendercontract.MandibleRig
	ConversationalMotion bool
	OralRecovery         bool
	EmotionRestOpening   *[10]float64
	Emotions             bool
	MotionScale          float64
	Expressive           bool
	Seed                 uint32
	BlinkSeed            uint32
	Blink                BlinkSettings
	Behavior             Behavior
	Breathing            *Breathing
	Bounds               Bounds
	FaceHeight           float64
	BreathFraction       float64
	BreathPeriod         float64
	Eyes                 [2]GazeEye
	Expressions          bool
	SmileAvailable       bool
	SpeakingSmile        bool
	LaughAvailable       bool
	ReferenceLaugh       bool
	Directed             bool
	Conversational       bool
}

// Load reads the unmodified prepared JSON. LoadBundle also binds the oral seed.
func Load(motionJSON, gazeJSON, expressionJSON []byte) (Config, error) {
	var m struct {
		Mandible             *rendercontract.MandibleRig `json:"mandible"`
		ConversationalMotion *struct{ Version string }   `json:"conversational_motion"`
		Seed                 uint32                      `json:"animation_seed"`
		Blink                BlinkSettings               `json:"blink"`
		Behavior             Behavior                    `json:"behavior"`
		Breathing            *Breathing                  `json:"breathing"`
		Projection           struct {
			Bounds Bounds `json:"bounds"`
		} `json:"projection"`
		Anatomy struct {
			FaceHeight float64 `json:"face_height"`
		} `json:"anatomy"`
		Motion struct {
			Fraction float64 `json:"breath_face_height_fraction"`
			Period   float64 `json:"breath_period_ms"`
		} `json:"motion"`
	}
	var g struct {
		Eyes []GazeEye `json:"eyes"`
	}
	var ex struct {
		EmotionRig *struct {
			RestOpening []float64 `json:"rest_opening"`
			Version     string
			Validation  struct{ Valid bool }
		} `json:"emotion_rig"`
		Smile     json.RawMessage `json:"smile"`
		Reference *struct {
			Presentation *struct{ Version string } `json:"presentation"`
			Oral         struct {
				Transition *struct{ Version string } `json:"transition"`
			}
			Version     string
			DentalReady bool `json:"dental_ready"`
			Validation  struct{ Valid bool }
		} `json:"reference_laughter"`
		Joint *struct {
			Version       string
			LaughterReady bool `json:"laughter_ready"`
			Laugh         struct {
				EyeClosure float64 `json:"eye_closure"`
			} `json:"laugh"`
			Validation struct{ Valid bool }
		} `json:"joint_articulation"`
	}
	if json.Unmarshal(motionJSON, &m) != nil || json.Unmarshal(gazeJSON, &g) != nil || (len(expressionJSON) > 0 && json.Unmarshal(expressionJSON, &ex) != nil) {
		return Config{}, errors.New("invalid prepared animation JSON")
	}
	if len(g.Eyes) != 2 || m.Anatomy.FaceHeight <= 0 || m.Projection.Bounds.Yaw <= 0 || m.Blink.Closing <= 0 || m.Blink.Opening <= 0 {
		return Config{}, errors.New("invalid prepared animation bounds")
	}
	if b := m.Breathing; b != nil {
		valid := func(r [2]float64, low, high float64) bool { return r[0] >= low && r[0] <= r[1] && r[1] <= high }
		if !valid(b.Duration, 1000, 15000) || !valid(b.Inhale, .1, .49) || !valid(b.Amplitude, 0, 1) {
			return Config{}, errors.New("breathing settings exceed the validated motion envelope")
		}
	}
	c := Config{Mandible: m.Mandible, Seed: m.Seed, BlinkSeed: m.Seed, Blink: m.Blink, Behavior: m.Behavior, Breathing: m.Breathing, Bounds: m.Projection.Bounds, FaceHeight: m.Anatomy.FaceHeight, BreathFraction: m.Motion.Fraction, BreathPeriod: m.Motion.Period, Expressions: len(expressionJSON) > 0, SmileAvailable: len(ex.Smile) > 0 && string(ex.Smile) != "null", Directed: true, Conversational: true}
	if m.ConversationalMotion != nil {
		if m.ConversationalMotion.Version != rendercontract.ConversationalMotionVersion {
			return Config{}, errors.New("invalid conversational motion profile")
		}
		c.ConversationalMotion = true
	}
	c.OralRecovery = ex.Reference != nil && ex.Reference.Oral.Transition != nil && ex.Reference.Oral.Transition.Version == "oral-transition/1"
	copy(c.Eyes[:], g.Eyes)
	c.SpeakingSmile = ex.Joint != nil && ex.Joint.Version == "joint-expression-articulation/1" && ex.Joint.Validation.Valid
	c.Emotions = ex.EmotionRig != nil && ex.EmotionRig.Version == "emotion-rig/1" && ex.EmotionRig.Validation.Valid
	if c.Emotions && ex.EmotionRig.RestOpening != nil {
		if len(ex.EmotionRig.RestOpening) != 10 {
			return Config{}, errors.New("invalid emotional rest opening length")
		}
		for i, value := range ex.EmotionRig.RestOpening {
			if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > .4 || (i == 0 && value != 0) {
				return Config{}, errors.New("invalid emotional rest opening")
			}
		}
		opening := [10]float64{}
		copy(opening[:], ex.EmotionRig.RestOpening)
		c.EmotionRestOpening = &opening
	}
	c.Expressive = c.SpeakingSmile
	c.LaughAvailable = c.SpeakingSmile && ex.Joint.LaughterReady
	c.ReferenceLaugh = ex.Reference != nil && ex.Reference.Version == "reference-laughter/1" && ex.Reference.Validation.Valid
	if c.ReferenceLaugh {
		c.LaughterV2 = ex.Reference.Presentation != nil && ex.Reference.Presentation.Version == "laughter-presentation/2"
		c.LaughAvailable = ex.Reference.DentalReady
	}
	return c, nil
}
func LoadBundle(motionJSON, gazeJSON, expressionJSON, oralJSON []byte) (Config, error) {
	c, err := Load(motionJSON, gazeJSON, expressionJSON)
	if err != nil {
		return c, err
	}
	var oral struct {
		BuildID string `json:"build_id"`
	}
	if json.Unmarshal(oralJSON, &oral) != nil || len(oral.BuildID) < 8 {
		return Config{}, errors.New("invalid oral animation seed")
	}
	seed, err := strconv.ParseUint(oral.BuildID[:8], 16, 32)
	if err != nil {
		return Config{}, errors.New("invalid oral animation seed")
	}
	c.BlinkSeed = uint32(seed)
	return c, nil
}
func clamp(v, low, high float64) float64 { return math.Max(low, math.Min(high, v)) }
func unit(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return clamp(v, 0, 1)
}
func ease(t float64) float64   { p := unit(t); return p * p * p * (10 + p*(-15+6*p)) }
func smooth(t float64) float64 { p := unit(t); return p * p * (3 - 2*p) }

type random struct{ state uint32 }

func (r *random) next() float64 {
	r.state = 1664525*r.state + 1013904223
	return float64(r.state) / 4294967296
}
func between(r *random, p [2]float64) float64 { return p[0] + r.next()*(p[1]-p[0]) }
func blend(a, b, t float64) float64           { return a + (b-a)*t }
func blendHead(a, b Head, t float64) Head {
	return Head{blend(a.Yaw, b.Yaw, t), blend(a.Pitch, b.Pitch, t), blend(a.Roll, b.Roll, t)}
}
