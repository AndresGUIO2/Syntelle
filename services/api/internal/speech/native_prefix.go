package speech

import "syntelle/render-contract"

// Multi-request phrases retain the existing complete composition path.
func (conversationSpeech) CanStreamPhrase(text string) bool { return len(splitSpeech(text)) == 1 }

// Native playback retains its separate 200 ms event lookahead. These thresholds
// control publication only; samples and provider event offsets remain original.
func nativePCMPrefix(input Input, base Result, pcm []byte, events []Viseme, words []rendercontract.WordBoundary, previousBytes int) (Result, int, bool) {
	n := len(pcm) / 2 * 2
	if len(events) < 2 {
		return Result{}, previousBytes, false
	}
	n = min(n, int(events[len(events)-1].OffsetMS*48)/2*2)
	if n < 28800 || n-previousBytes < 9600 {
		return Result{}, previousBytes, false
	}
	prefix := base
	prefix.Audio = pcmWAV(pcm[:n])
	prefix.DurationMS = float64(n) / 48
	prefix.Visemes = nil
	for _, v := range events {
		if v.OffsetMS <= prefix.DurationMS {
			prefix.Visemes = append(prefix.Visemes, v)
		}
	}
	completedWords := make([]rendercontract.WordBoundary, 0, len(words))
	for _, w := range words {
		if w.OffsetMS+w.DurationMS <= prefix.DurationMS {
			completedWords = append(completedWords, w)
		}
	}
	prefix.Words = normalizeWords(input, completedWords, prefix.DurationMS)
	return prefix, n, len(prefix.Visemes) >= 2
}
