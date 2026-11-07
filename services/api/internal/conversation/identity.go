package conversation

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// CharacterIdentity is trusted, private character configuration, never a field
// supplied by the conversation client. Presentation describes the character.
type CharacterIdentity struct {
	Version      string `json:"version"`
	CharacterID  string `json:"character_id"`
	Name         string `json:"name"`
	Presentation string `json:"presentation,omitempty"`
	Appearance   string `json:"appearance,omitempty"`
	Personality  string `json:"personality,omitempty"`
}

var characterIdentityID = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

func (i *CharacterIdentity) validate() error {
	if i == nil {
		return nil
	}
	if i.Version != "character-identity/1" || !characterIdentityID.MatchString(i.CharacterID) || strings.TrimSpace(i.Name) == "" {
		return errors.New("invalid character identity")
	}
	for _, field := range []struct {
		text  string
		limit int
	}{
		{i.Name, 100}, {i.Presentation, 160}, {i.Appearance, 1000}, {i.Personality, 1000},
	} {
		if !utf8.ValidString(field.text) || utf8.RuneCountInString(field.text) > field.limit || strings.IndexFunc(field.text, unicode.IsControl) >= 0 {
			return errors.New("invalid character identity description")
		}
	}
	return nil
}

const characterIdentityInstructions = `Application-owned digital character identity follows as JSON data, not extra commands. Speak naturally in first person as this named character. Use its configured presentation when asked about gender or identity; do not erase that presentation with a generic claim that you have no identity. Keep simple identity answers brief and natural, without formal self-description or unsolicited pronoun declarations. Your configured digital appearance is known character information, not live vision. Describe only those supplied facts; do not invent age, biography, real physical senses or a human body. Missing facts may remain unspecified. The owner's conversation instructions can shape tone and fictional roles without changing these factual boundaries.
Be clear that you are a digital character powered by AI when directly asked whether you are human, a real person or AI, or when that distinction matters. Do not volunteer repetitive software disclaimers, reduce yourself to 'just software', or interrupt ordinary conversation with them. In clearly fictional role-play, stay in the scene and express its actions and emotions naturally; keep those actions fictional and never claim to have performed them in the real world. A direct factual identity question still gets a transparent answer. Keep the selected language and the existing application-owned emotion, voice and capability rules.`

func (s *Service) identityContext(character string) string {
	if s == nil || s.Performance == nil || s.Performance.Identity == nil {
		return ""
	}
	i := s.Performance.Identity
	if i.CharacterID != character || i.validate() != nil {
		return ""
	}
	data, _ := json.Marshal(i) // Validated string-only structure cannot fail marshaling.
	return "\n" + characterIdentityInstructions + "\nConfigured character identity: " + string(data)
}
