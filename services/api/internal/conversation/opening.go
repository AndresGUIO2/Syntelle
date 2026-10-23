package conversation

import (
	"encoding/json"
	"strings"
	"unicode/utf8"
)

// OpeningPhrase is shared by speculative preparation and final segmentation.
// A growing string is admitted only at a complete sentence or stable word cut.
func OpeningPhrase(text string, complete bool) string {
	units, cut, word := 0, 0, 0
	for index, r := range text {
		n := 1
		if r > 0xffff {
			n = 2
		}
		if units+n > 220 {
			if word > 0 {
				cut = word
			}
			return strings.TrimSpace(text[:cut])
		}
		units += n
		cut = index + len(string(r))
		if r == ' ' || r == '\n' {
			word = cut
		}
		if strings.ContainsRune(".!?。！？", r) && units >= 24 {
			// Wait for a following character so a fragmented decimal/abbreviation is
			// not classified from a punctuation byte alone.
			if cut < len(text) {
				next, _ := utf8.DecodeRuneInString(text[cut:])
				if next == ' ' || next == '\n' || next == '\r' || next == '\t' {
					return strings.TrimSpace(text[:cut])
				}
			} else if complete {
				return strings.TrimSpace(text[:cut])
			}
		}
	}
	if complete {
		return strings.TrimSpace(text)
	}
	return ""
}

// Extract only a decoded prefix of the top-level text string. An unfinished
// escape, surrogate pair or UTF-8 sequence never becomes speech input.
func decodedTextPrefix(data string) string {
	d := json.NewDecoder(strings.NewReader(data))
	t, err := d.Token()
	if err != nil || t != json.Delim('{') {
		return ""
	}
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return ""
		}
		if key != "text" {
			var skip json.RawMessage
			if d.Decode(&skip) != nil {
				return ""
			}
			continue
		}
		raw := strings.TrimLeft(data[d.InputOffset():], " \t\r\n:")
		if len(raw) == 0 || raw[0] != '"' {
			return ""
		}
		escaped, closed := false, false
		end := len(raw)
		for i := 1; i < len(raw); i++ {
			if escaped {
				escaped = false
				continue
			}
			if raw[i] == '\\' {
				escaped = true
				continue
			}
			if raw[i] == '"' {
				end = i + 1
				closed = true
				break
			}
		}
		encoded := raw[:end]
		if !closed {
			encoded += "\""
		}
		var prefix string
		if json.Unmarshal([]byte(encoded), &prefix) != nil || !utf8.ValidString(prefix) || strings.ContainsRune(prefix, utf8.RuneError) || strings.ContainsAny(prefix, "[]{}<>") {
			return ""
		}
		return prefix
	}
	return ""
}
