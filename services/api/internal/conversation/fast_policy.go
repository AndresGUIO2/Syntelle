package conversation

import (
	"encoding/binary"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	fastEndpointVersion   = "audio-quiet/2"
	legacyEndpointVersion = "audio-quiet/1"
	fastSegmentationMS    = 450
	fastQuiet             = 650 * time.Millisecond
	fastFinalGuard        = 100 * time.Millisecond
	fastOpenClauseQuiet   = 900 * time.Millisecond
	fastFallback          = 250 * time.Millisecond
	fastPCMLease          = 750 * time.Millisecond
	fastSDKLease          = 1000 * time.Millisecond
)

// Shared with the native Azure adapter; this is not a public request option.
const RecognitionSegmentationMS = fastSegmentationMS

// Socket-owned. The ring contains RMS scalars; microphone bytes are discarded.
// Energy gets one bounded continuation lease per confirmed epoch. Only fresh
// SDK progress may extend the SDK lease.
type fastTurnPolicy struct {
	samples, voiceSample, confirmedSample int64
	at, voiceAt, finalAt                  time.Time
	run, quietRun                         int64
	active, seen                          bool
	noise                                 [100]float64
	noiseCount, noiseNext, noiseStep      int
	floor                                 float64
	pcmAt, sdkAt                          time.Time
	sdkPending                            bool
	sdkText                               string
	sdkEnd, sdkStart                      int64
}

func (q *fastTurnPolicy) audio(pcm []byte, now time.Time) {
	for at := 0; at+1 < len(pcm); {
		end := min(at+640, len(pcm))
		n := (end - at) / 2
		sum, square := 0., 0.
		for i := at; i+1 < end; i += 2 {
			v := float64(int16(binary.LittleEndian.Uint16(pcm[i:]))) / 32768
			sum += v
			square += v * v
		}
		mean := sum / float64(n)
		rms := math.Sqrt(math.Max(0, square/float64(n)-mean*mean))
		q.samples += int64(n)
		q.noise[q.noiseNext] = rms
		q.noiseNext = (q.noiseNext + 1) % len(q.noise)
		q.noiseCount = min(len(q.noise), q.noiseCount+1)
		q.noiseStep++
		if q.floor == 0 {
			q.floor = .003
		}
		if q.noiseCount >= 10 && q.noiseStep >= 5 {
			values := append([]float64(nil), q.noise[:q.noiseCount]...)
			sort.Float64s(values)
			q.floor = math.Max(.001, math.Min(.025, values[len(values)/5]))
			q.noiseStep = 0
		}
		threshold := math.Max(.012, q.floor*3)
		if q.active {
			threshold = math.Max(.008, q.floor*1.8)
		}
		if rms >= threshold {
			q.run += int64(n)
			q.quietRun = 0
			if !q.finalAt.IsZero() && q.samples > q.confirmedSample+1920 && q.pcmAt.IsZero() {
				q.pcmAt = now
			}
			if q.run >= 1280 {
				q.active = true
				q.seen = true
				q.voiceSample = q.samples
				q.voiceAt = now
			}
		} else {
			q.run = 0
			q.quietRun += int64(n)
			if q.quietRun >= 1280 {
				q.active = false
			}
		}
		at = end
	}
	q.at = now
}

func (q *fastTurnPolicy) confirm(e RecognitionEvent, now time.Time) {
	q.finalAt = now
	if end := e.OffsetMS + e.DurationMS; end > 0 && !math.IsNaN(end) && !math.IsInf(end, 0) {
		q.confirmedSample = int64(end * 16)
	} else {
		q.confirmedSample = q.samples
	}
	q.pcmAt, q.sdkAt = time.Time{}, time.Time{}
	q.sdkPending = false
	q.sdkText = ""
	q.sdkEnd = q.confirmedSample
	q.sdkStart = -1
	if q.voiceSample > q.confirmedSample+1920 && now.Sub(q.voiceAt) < 300*time.Millisecond {
		q.pcmAt = now
	}
}

func (q *fastTurnPolicy) progress(e RecognitionEvent, now time.Time) bool {
	if q.finalAt.IsZero() {
		return false
	}
	end := e.OffsetMS + e.DurationMS
	if math.IsNaN(end) || math.IsInf(end, 0) || end < 0 {
		return false
	}
	hasSource := end > 0
	sample := int64(end * 16)
	if hasSource && sample <= q.confirmedSample+320 {
		return false
	}
	switch e.Type {
	case "end":
		q.sdkPending = false
		return false
	case "start":
		start := int64(e.OffsetMS * 16)
		if !hasSource || start <= q.sdkStart {
			return false
		}
		q.sdkStart = start
	case "partial":
		text := strings.TrimSpace(e.Text)
		if text == "" || text == q.sdkText && (!hasSource || sample <= q.sdkEnd) {
			return false
		}
		if hasSource && sample < q.sdkEnd {
			return false
		}
		q.sdkText = text
		q.sdkEnd = max(q.sdkEnd, sample)
	default:
		return false
	}
	q.sdkAt = now
	q.sdkPending = true
	return true
}

func (q *fastTurnPolicy) clearEpoch() {
	q.finalAt, q.pcmAt, q.sdkAt = time.Time{}, time.Time{}, time.Time{}
	q.sdkPending = false
	q.sdkText = ""
	q.confirmedSample = 0
	q.sdkEnd = 0
	q.sdkStart = -1
}

// Called on a copy at the timer's finite input watermark. Silent packets and
// settings still run in FIFO order after release. Only a turn-replacing control
// or actual post-final energy can require handling them before submission.
func (q *fastTurnPolicy) queuedTurnEvidence(e Event, mic string, now time.Time) bool {
	switch e.Type {
	case "mic_stop", "mic_start", "cancel", "submit", "speak":
		return true
	case "audio":
		if e.TurnID == mic && len(e.Audio) != 0 && len(e.Audio) <= 6400 && len(e.Audio)%2 == 0 {
			q.audio(e.Audio, now)
			return !q.pcmAt.IsZero() && now.Before(q.finalAt.Add(fastPCMLease))
		}
	}
	return false
}
func (q *fastTurnPolicy) expired(now time.Time) bool {
	if q.sdkPending {
		return now.Sub(q.sdkAt) >= fastSDKLease
	}
	return !q.pcmAt.IsZero() && now.Sub(q.finalAt) >= fastPCMLease
}
func (q *fastTurnPolicy) wait(text string, now time.Time) (time.Duration, string) {
	if q.finalAt.IsZero() {
		return 0, "no_confirmed_text"
	}
	guard := max(0, fastFinalGuard-now.Sub(q.finalAt))
	if q.sdkPending {
		left := fastSDKLease - now.Sub(q.sdkAt)
		if left <= 0 {
			return guard, "stalled_sdk_progress"
		}
		return max(guard, left), "sdk_progress_lease"
	}
	if !q.pcmAt.IsZero() {
		left := fastPCMLease - now.Sub(q.finalAt)
		if left <= 0 {
			return guard, "pcm_only_lease_expired"
		}
		if q.voiceSample > q.confirmedSample+1920 || q.run > 0 {
			return max(guard, left), "pcm_only_continuation"
		}
	}
	if !q.seen || q.at.IsZero() || now.Sub(q.at) >= fastFallback {
		return max(guard, max(0, fastFallback-now.Sub(q.finalAt))), "stale_activity_fallback"
	}
	quiet := time.Duration(q.samples-q.voiceSample) * time.Second / 16000
	target := fastQuiet
	if unfinishedClause(text) {
		target = fastOpenClauseQuiet
	}
	// Reevaluate when fresh PCM becomes stale, even if no more packets arrive.
	return max(guard, min(max(0, target-quiet), fastFallback-now.Sub(q.at))), "observed_quiet"
}

type turnEndpoint struct {
	legacy turnQuiet
	fast   *fastTurnPolicy
}

func newTurnEndpoint(version string) turnEndpoint {
	if version == legacyEndpointVersion {
		return turnEndpoint{}
	}
	return turnEndpoint{fast: &fastTurnPolicy{sdkStart: -1}}
}
func (q *turnEndpoint) audio(pcm []byte, now time.Time) {
	if q.fast != nil {
		q.fast.audio(pcm, now)
	} else {
		q.legacy.audio(pcm, now)
	}
}
func (q *turnEndpoint) delay(text string, now time.Time) time.Duration {
	if q.fast != nil {
		d, _ := q.fast.wait(text, now)
		return d
	}
	return q.legacy.delay(text, now)
}
func (q *turnEndpoint) confirm(e RecognitionEvent, now time.Time) {
	if q.fast != nil {
		q.fast.confirm(e, now)
	} else {
		q.legacy.confirm(e.OffsetMS + e.DurationMS)
	}
}
func (q *turnEndpoint) progress(e RecognitionEvent, now time.Time) bool {
	if q.fast != nil {
		return q.fast.progress(e, now)
	}
	return true
}
func (q *turnEndpoint) recentVoice(now time.Time) bool {
	if q.fast != nil {
		return q.fast.active && now.Sub(q.fast.voiceAt) < 300*time.Millisecond
	}
	return q.legacy.recentVoice(now)
}
func (q *turnEndpoint) pendingVoice() bool {
	if q.fast != nil {
		return false
	}
	return q.legacy.pendingVoice()
}
func (q *turnEndpoint) clearEpoch() {
	if q.fast != nil {
		q.fast.clearEpoch()
	}
}

type lateRecognitionWindow struct {
	startMS, endMS float64
	until          time.Time
}

func lateConfirmedBeforePlayback(e RecognitionEvent, late []lateRecognitionWindow, windows []playbackWindow, now time.Time) bool {
	if len(windows) == 0 || e.DurationMS <= 0 {
		return false
	}
	end := e.OffsetMS + e.DurationMS
	if math.IsNaN(end) || math.IsInf(end, 0) || end > windows[len(windows)-1].start {
		return false
	}
	for _, playback := range windows {
		if end > playback.start && (playback.end < 0 || e.OffsetMS < playback.end) {
			return false
		}
	}
	for _, w := range late {
		if now.Before(w.until) && e.OffsetMS >= w.startMS-120 && end > w.startMS && end <= w.endMS+20 {
			return true
		}
	}
	return false
}
