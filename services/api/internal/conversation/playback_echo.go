package conversation

import (
	"strings"
	"unicode"
)

// Compare only observed speech text; short commands remain valid interruptions.
// This suppresses verbatim speaker leakage, not arbitrary physical-room echoes.
func matchesPlayback(recognized, spoken string) bool {
	words := func(value string) []string {
		return strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsNumber(r)
		})
	}
	input := words(recognized)
	if len(input) < 4 {
		return false
	}
	return strings.Contains(" "+strings.Join(words(spoken), " ")+" ", " "+strings.Join(input, " ")+" ")
}
