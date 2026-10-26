package rendercontract

import (
	"errors"
	"math"
	"strings"
)

type Settings struct {
	EmotionBaseline      EmotionBaseline   `json:"emotion_baseline"`
	EmotionRegulation    EmotionRegulation `json:"emotion_regulation"`
	Character            string            `json:"character"`
	Instructions         string            `json:"instructions"`
	Language             string            `json:"language"`
	Voice                string            `json:"voice"`
	Background           string            `json:"background"`
	Blur                 float64           `json:"blur"`
	Blinking             bool              `json:"blinking"`
	Movement             bool              `json:"movement"`
	AutomaticExpressions bool              `json:"automatic_expressions"`
	Smile                float64           `json:"smile"`
	Brows                float64           `json:"brows"`
	ReducedMotion        bool              `json:"reduced_motion,omitempty"`
}

func Defaults() Settings {
	return Settings{
		EmotionBaseline: EmotionBaseline{State: "neutral"},
		Character:       "syntelle", Language: "en-US", Voice: "en-US-Ava:DragonHDLatestNeural",
		Background: "light-studio", Blinking: true, Movement: true, AutomaticExpressions: true,
		Instructions: "Be a clear, friendly assistant. Match the depth and length to the question, and explain fully when asked. Do not invent company knowledge.",
	}
}

func (s Settings) Validate() error {
	if !s.EmotionBaseline.Valid() {
		return errors.New("choose a supported emotion and intensity")
	}
	if !s.EmotionRegulation.Valid() {
		return errors.New("choose a supported temperament")
	}
	if s.Character != "syntelle" || (s.Language != "en-US" && s.Language != "es-CO") {
		return errors.New("choose Syntelle and a supported language")
	}
	if s.Voice != "en-US-Ava:DragonHDLatestNeural" && s.Voice != "en-US-Andrew:DragonHDLatestNeural" {
		return errors.New("choose a supported voice")
	}
	if s.Background != "light-studio" && s.Background != "dark-lounge" && s.Background != "natural-space" {
		return errors.New("choose a supplied background")
	}
	if len([]rune(s.Instructions)) > 2000 || strings.TrimSpace(s.Instructions) == "" {
		return errors.New("instructions must contain between 1 and 2,000 characters")
	}
	for _, v := range []float64{s.Blur, s.Smile, s.Brows} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return errors.New("settings must contain finite numbers")
		}
	}
	if s.Blur < 0 || s.Blur > 20 || s.Smile < 0 || s.Smile > 1 || s.Brows < 0 || s.Brows > 1 {
		return errors.New("settings are outside the supported range")
	}
	return nil
}
