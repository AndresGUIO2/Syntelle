package nativemedia

import (
	"context"
	"syntelle/render-contract"
	"testing"
)

func TestPreviewEndWaitsForSuccessfulSettledFrame(t *testing.T) {
	var events []Envelope
	s := session{generation: 3, previewTurn: "preview", host: &Host{ctx: context.Background(), config: Config{Emit: func(e rendercontract.Envelope) { events = append(events, e) }}}}
	if !s.deferPreviewEnd(playbackNotification{"playback_ended", "preview"}, 3) {
		t.Fatal("preview ended at PCM boundary")
	}
	s.finishPreviewPresentation(3, true)
	if len(events) != 0 {
		t.Fatal("recovery still visible")
	}
	s.paused = true
	s.finishPreviewPresentation(3, false)
	if len(events) != 0 {
		t.Fatal("pause released presentation")
	}
	s.paused = false
	s.finishPreviewPresentation(2, false)
	if len(events) != 0 {
		t.Fatal("stale frame released presentation")
	}
	s.finishPreviewPresentation(3, false)
	s.finishPreviewPresentation(3, false)
	if len(events) != 1 {
		t.Fatal("missing or duplicate final event", events)
	}
}
