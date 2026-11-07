package conversation

import (
	"encoding/binary"
	"fmt"
	"testing"
	"time"
)

func fastPCM(amplitude, dc int16) []byte {
	b := make([]byte, 640)
	for i := 0; i < 320; i++ {
		v := int(dc)
		if i%16 < 8 {
			v += int(amplitude)
		} else {
			v -= int(amplitude)
		}
		binary.LittleEndian.PutUint16(b[i*2:], uint16(int16(v)))
	}
	return b
}
func fastFeed(q *fastTurnPolicy, now *time.Time, n int, amplitude, dc int16) {
	for range n {
		*now = now.Add(20 * time.Millisecond)
		q.audio(fastPCM(amplitude, dc), *now)
	}
}
func TestFastPolicyReleasesConfirmedQuietWithoutSecondSilence(t *testing.T) {
	q := fastTurnPolicy{}
	now := time.Unix(1, 0)
	fastFeed(&q, &now, 25, 2000, 0)
	fastFeed(&q, &now, 35, 0, 0)
	q.confirm(RecognitionEvent{DurationMS: 500}, now)
	if wait, _ := q.wait("What time is it?", now); wait != 100*time.Millisecond {
		t.Fatalf("extra silence after confirmed quiet: %v", wait)
	}
	if wait, _ := q.wait("What time is it?", now.Add(101*time.Millisecond)); wait != 0 {
		t.Fatal("final guard did not expire", wait)
	}
}
func TestFastPolicyFallbackAndOpenClauseAreBounded(t *testing.T) {
	t.Run("missing local PCM", func(t *testing.T) {
		q := fastTurnPolicy{}
		now := time.Unix(1, 0)
		q.confirm(RecognitionEvent{DurationMS: 500}, now)
		if d, _ := q.wait("Ready.", now); d != 250*time.Millisecond {
			t.Fatal("fallback", d)
		}
	})
	t.Run("open clause", func(t *testing.T) {
		q := fastTurnPolicy{}
		now := time.Unix(1, 0)
		fastFeed(&q, &now, 25, 2000, 0)
		fastFeed(&q, &now, 25, 0, 0)
		q.confirm(RecognitionEvent{DurationMS: 500}, now)
		if d, _ := q.wait("I was thinking about", now); d > 400*time.Millisecond || d < 100*time.Millisecond {
			t.Fatal("open clause quiet", d)
		}
	})
}
func TestFastPolicyDCAndStationaryNoiseCannotHoldFinal(t *testing.T) {
	for _, tc := range []struct {
		name          string
		amplitude, dc int16
	}{{"DC bias", 0, 9000}, {"stationary AC noise plus DC", 700, 5000}} {
		t.Run(tc.name, func(t *testing.T) {
			q := fastTurnPolicy{}
			now := time.Unix(1, 0)
			fastFeed(&q, &now, 100, tc.amplitude, tc.dc)
			q.confirm(RecognitionEvent{DurationMS: 1000}, now)
			for range 100 {
				fastFeed(&q, &now, 1, tc.amplitude, tc.dc)
			}
			if d, reason := q.wait("Can you tell me a story?", now); d != 0 {
				t.Fatalf("noise kept confirmed text pending: %v %s", d, reason)
			}
		})
	}
}
func TestFastPolicyPCMOnlyContinuationHasHardLease(t *testing.T) {
	q := fastTurnPolicy{}
	now := time.Unix(1, 0)
	fastFeed(&q, &now, 25, 2200, 0)
	fastFeed(&q, &now, 35, 0, 0)
	q.confirm(RecognitionEvent{DurationMS: 500}, now)
	fastFeed(&q, &now, 10, 2500, 0)
	if d, _ := q.wait("A confirmed fragment.", now); d <= 0 {
		t.Fatal("immediate continuation cut")
	}
	fastFeed(&q, &now, 45, 2500, 0)
	if d, reason := q.wait("A confirmed fragment.", now); d != 0 {
		t.Fatal("PCM-only continuation renewed its own lease", d, reason)
	}
}
func TestFastPolicySDKProgressProtectsLongContinuation(t *testing.T) {
	q := fastTurnPolicy{}
	now := time.Unix(1, 0)
	fastFeed(&q, &now, 25, 2200, 0)
	fastFeed(&q, &now, 35, 0, 0)
	q.confirm(RecognitionEvent{DurationMS: 500}, now)
	for i := 0; i < 22; i++ {
		fastFeed(&q, &now, 10, 2500, 0)
		q.progress(RecognitionEvent{Type: "partial", Text: fmt.Sprintf("In my daily routine %d", i), OffsetMS: 1200, DurationMS: float64((i + 1) * 200)}, now)
		if d, _ := q.wait("A confirmed fragment.", now); d <= 0 {
			t.Fatal("advancing SDK continuation was cut", i)
		}
	}
	fastFeed(&q, &now, 55, 0, 0)
	if d, _ := q.wait("A confirmed fragment.", now); d != 0 {
		t.Fatal("stalled SDK progress did not expire", d)
	}
}
func TestFastPolicyDuplicateAndStaleSDKDoNotRenewLease(t *testing.T) {
	q := fastTurnPolicy{}
	now := time.Unix(1, 0)
	q.confirm(RecognitionEvent{OffsetMS: 1000, DurationMS: 500}, now)
	if q.progress(RecognitionEvent{Type: "partial", Text: "old callback", OffsetMS: 0, DurationMS: 500}, now) {
		t.Fatal("stale segment extended epoch")
	}
	e := RecognitionEvent{Type: "partial", Text: "new continuation", OffsetMS: 1600, DurationMS: 100}
	if !q.progress(e, now) {
		t.Fatal("fresh SDK progress ignored")
	}
	if d, _ := q.wait("Confirmed.", now); d != time.Second {
		t.Fatal("SDK lease", d)
	}
	for i := 0; i < 8; i++ {
		now = now.Add(200 * time.Millisecond)
		if q.progress(e, now) {
			t.Fatal("identical partial counted as progress")
		}
	}
	if d, _ := q.wait("Confirmed.", now); d != 0 {
		t.Fatal("duplicate partial retained confirmed turn", d)
	}
	q.clearEpoch()
	if d, _ := q.wait("", now); d != 0 {
		t.Fatal("flush retained old epoch", d)
	}
}

func TestFastPolicySourceProgressAndReplayedStarts(t *testing.T) {
	q := fastTurnPolicy{}
	now := time.Unix(1, 0)
	q.confirm(RecognitionEvent{DurationMS: 500}, now)
	start := RecognitionEvent{Type: "start", OffsetMS: 700}
	if !q.progress(start, now) {
		t.Fatal("new SDK source start ignored")
	}
	q.progress(RecognitionEvent{Type: "end"}, now)
	if q.progress(start, now.Add(time.Second)) {
		t.Fatal("replayed SDK start renewed a resolved source span")
	}
	partial := RecognitionEvent{Type: "partial", Text: "a sustained word", OffsetMS: 700, DurationMS: 200}
	q.progress(partial, now)
	partial.DurationMS = 400
	if !q.progress(partial, now.Add(500*time.Millisecond)) {
		t.Fatal("genuinely advancing source span ignored")
	}
	if wait, _ := q.wait("Confirmed.", now.Add(1400*time.Millisecond)); wait != 100*time.Millisecond {
		t.Fatal("SDK source progression did not own its bounded lease", wait)
	}
	if wait, reason := q.wait("Confirmed.", now.Add(1500*time.Millisecond)); wait != 0 || reason != "stalled_sdk_progress" {
		t.Fatal("SDK lease did not report its exact bounded expiry", wait, reason)
	}
}

func TestFastPolicyPCMLeaseIsNotAddedToSDKDeadline(t *testing.T) {
	q := fastTurnPolicy{}
	now := time.Unix(1, 0)
	q.confirm(RecognitionEvent{DurationMS: 500}, now)
	q.pcmAt = now
	q.progress(RecognitionEvent{Type: "partial", Text: "continuing", OffsetMS: 700, DurationMS: 100}, now)
	if wait, _ := q.wait("Confirmed.", now.Add(time.Second)); wait != 0 {
		t.Fatal("PCM time was added after the SDK lease", wait)
	}
}

func TestFastPolicyLateFirstEnergyDoesNotMoveFinalDeadline(t *testing.T) {
	q := fastTurnPolicy{}
	now := time.Unix(1, 0)
	fastFeed(&q, &now, 25, 3500, 0)
	fastFeed(&q, &now, 35, 0, 0)
	q.confirm(RecognitionEvent{DurationMS: 500}, now)
	q.audio(fastPCM(3500, 0), now.Add(500*time.Millisecond))
	if wait, _ := q.wait("Confirmed.", now.Add(500*time.Millisecond)); wait != 250*time.Millisecond {
		t.Fatal("late acoustic evidence moved the post-final deadline", wait)
	}
	if wait, reason := q.wait("Confirmed.", now.Add(750*time.Millisecond)); wait != 0 || reason != "pcm_only_lease_expired" {
		t.Fatal("late acoustic evidence received an additive lease", wait, reason)
	}
}

func TestReadyTurnProcessesQueuedSilenceAndSettingsAfterRelease(t *testing.T) {
	q := fastTurnPolicy{}
	now := time.Unix(1, 0)
	fastFeed(&q, &now, 25, 3500, 0)
	fastFeed(&q, &now, 35, 0, 0)
	q.confirm(RecognitionEvent{DurationMS: 500}, now)
	now = now.Add(fastFinalGuard)
	probe := q
	for range 30 {
		for _, e := range []Event{{Type: "audio", TurnID: "mic", Audio: fastPCM(0, 9000)}, {Type: "configure"}, {Type: "activity"}} {
			if probe.queuedTurnEvidence(e, "mic", now) {
				t.Fatalf("background %s vetoed a ready turn", e.Type)
			}
		}
	}
	if q.samples == probe.samples || !probe.queuedTurnEvidence(Event{Type: "audio", TurnID: "mic", Audio: fastPCM(3500, 0)}, "mic", now) {
		t.Fatal("finite lookahead lost genuine continuation or counted PCM twice")
	}
	if !probe.queuedTurnEvidence(Event{Type: "mic_stop"}, "mic", now) {
		t.Fatal("microphone replacement must run before release")
	}
}

func TestFastPolicyLateSourceWindowDoesNotAdmitPlaybackEcho(t *testing.T) {
	now := time.Unix(1, 0)
	late := []lateRecognitionWindow{{500, 1800, now.Add(30 * time.Second)}}
	windows := []playbackWindow{{start: 2000, end: -1}}
	cases := []struct {
		name string
		e    RecognitionEvent
		want bool
	}{
		{"bounded pre-playback continuation", RecognitionEvent{OffsetMS: 1200, DurationMS: 500}, true},
		{"playback overlap", RecognitionEvent{OffsetMS: 1700, DurationMS: 500}, false},
		{"outside recorded unresolved source", RecognitionEvent{OffsetMS: 1800, DurationMS: 100}, false},
		{"old confirmed source", RecognitionEvent{OffsetMS: 0, DurationMS: 500}, false},
		{"unknown source interval", RecognitionEvent{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := lateConfirmedBeforePlayback(tc.e, late, windows, now); got != tc.want {
				t.Fatal("late source admission", got, tc.want)
			}
		})
	}
	if lateConfirmedBeforePlayback(cases[0].e, late, windows, now.Add(31*time.Second)) {
		t.Fatal("expired late source retained")
	}
	previous := []playbackWindow{{start: 1100, end: 1600}, {start: 2000, end: -1}}
	if lateConfirmedBeforePlayback(cases[0].e, late, previous, now) {
		t.Fatal("late callback was incorrectly exempted from an earlier playback interval")
	}
}

func legacyEndpointService(store *Store, model Model, speech Speech, recognition Recognition) *Service {
	s := New(store, model, speech, recognition)
	s.endpointVersion = legacyEndpointVersion
	return s
}
