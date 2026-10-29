package conversation

import (
	"encoding/binary"
	"math"
	"strings"
	"time"
	"unicode"
)

// Socket-owned. PCM is inspected and discarded, never retained in this tracker.
// Local activity only postpones a turn; it never supplies recognized words.
type turnQuiet struct {
	seen                      bool
	at, voiceAt               time.Time
	samples, voiceSample, run int64
	confirmedSample           int64
	confirmed                 bool
}

func (q *turnQuiet) confirm(endMS float64) {
	if endMS > 0 {
		q.confirmed = true
		q.confirmedSample = int64(endMS * 16)
	}
}
func (q turnQuiet) pendingVoice() bool { return q.confirmed && q.voiceSample > q.confirmedSample+1920 }

func (q *turnQuiet) audio(pcm []byte, now time.Time) {
	// Process 20 ms windows even when transport batches several frames.
	for at := 0; at+1 < len(pcm); {
		end := min(at+640, len(pcm))
		n := (end - at) / 2
		energy := 0.0
		for i := at; i+1 < end; i += 2 {
			v := float64(int16(binary.LittleEndian.Uint16(pcm[i:]))) / 32768
			energy += v * v
		}
		q.samples += int64(n)
		if math.Sqrt(energy/float64(n)) >= 0.012 {
			q.run += int64(n)
			if q.run >= 1280 {
				q.seen = true
				q.voiceSample = q.samples
				q.voiceAt = now
			}
		} else {
			q.run = 0
		}
		at = end
	}
	q.at = now
}

func (q turnQuiet) recentVoice(now time.Time) bool {
	// Even the first candidate frame postpones a deadline. Requiring the
	// sustained-voice latch here would leave an 80 ms race at speech onset.
	return (q.run > 0 && now.Sub(q.at) < 300*time.Millisecond) || (q.seen && now.Sub(q.voiceAt) < 300*time.Millisecond)
}

func (q turnQuiet) delay(text string, now time.Time) time.Duration {
	if !q.seen || now.Sub(q.at) > 300*time.Millisecond {
		return utterancePause
	}
	quiet := time.Duration(q.samples-q.voiceSample) * time.Second / 16000
	target := 1100 * time.Millisecond
	if unfinishedClause(text) {
		target = 2500 * time.Millisecond
	}
	return max(250*time.Millisecond, target-quiet)
}

// This deliberately only detects obvious open clauses in the two supported
// languages. Punctuation alone is not semantic certainty from recognition.
func unfinishedClause(text string) bool {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) })
	if len(words) == 0 {
		return true
	}
	switch words[len(words)-1] {
	case "and", "or", "but", "because", "if", "when", "while", "with", "without", "about", "to", "the", "a", "an", "of", "for", "that",
		"y", "o", "pero", "porque", "si", "cuando", "mientras", "con", "sin", "sobre", "de", "del", "para", "que", "el", "la", "los", "las", "un", "una":
		return true
	}
	return strings.HasSuffix(strings.TrimSpace(text), "...") || strings.HasSuffix(strings.TrimSpace(text), "…")
}
