package conversation

// Recognition offsets use the PCM sample clock, not SDK callback arrival time.
// A final echo segment can arrive after playback has stopped. Keep its overlap
// ownership until it is evaluated, instead of relying on a 'playing' boolean.
type playbackWindow struct{ start, end float64 }

func overlapsPlayback(event RecognitionEvent, windows []playbackWindow, clock, activity float64) bool {
	end := event.OffsetMS + event.DurationMS
	for _, window := range windows {
		stop := window.end
		if stop < 0 {
			stop = clock
		}
		if end > window.start && event.OffsetMS < stop {
			if activity < event.OffsetMS-200 || activity > end+300 {
				return true
			}
		}
	}
	return false
}
