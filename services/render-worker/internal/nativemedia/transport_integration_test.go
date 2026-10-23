//go:build integration && nativecodecs && cgo

package nativemedia

import (
	"context"
	"encoding/json"
	"image"
	"sync"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"
	"syntelle/render-contract"
)

type fixtureScene struct{}

func (fixtureScene) Render(ctx context.Context, input FrameInput, dst *image.RGBA) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for i := 0; i < len(dst.Pix); i += 4 {
		dst.Pix[i] = 100
		dst.Pix[i+1] = 120
		dst.Pix[i+2] = 180
		dst.Pix[i+3] = 255
	}
	return nil
}

func TestNativePeerDeliversRTPAndReleasesSession(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	events := make(chan Envelope, 64)
	host, err := New(ctx, Config{Capacity: 1, PackageHash: "fixture-only", SceneFactory: func(rendercontract.Settings) (Scene, error) { return fixtureScene{}, nil }, Emit: func(e Envelope) {
		select {
		case events <- e:
		case <-ctx.Done():
		}
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := host.Close(); err != nil {
			t.Error(err)
		}
	}()
	peer, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatal(err)
	}
	var readers sync.WaitGroup
	defer func() {
		if err := peer.Close(); err != nil {
			t.Error(err)
		}
		readers.Wait()
	}()
	packets := make(chan string, 64)
	peer.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				_, _, err := track.ReadRTP()
				if err != nil {
					return
				}
				select {
				case packets <- track.Kind().String():
				case <-ctx.Done():
					return
				}
			}
		}()
	})
	peer.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}
		data, _ := json.Marshal(map[string]any{"candidate": candidate.ToJSON()})
		if err := host.Dispatch(ctx, Envelope{Type: "signal", SessionID: "fixture_native", Data: data}); err != nil && ctx.Err() == nil {
			t.Error(err)
		}
	})
	if err := host.Dispatch(ctx, Envelope{Type: "create", SessionID: "fixture_native", Data: []byte(`{"settings":{}}`)}); err != nil {
		t.Fatal(err)
	}
	if err := host.Dispatch(ctx, Envelope{Type: "create", SessionID: "fixture_second", Data: []byte(`{"settings":{}}`)}); err == nil {
		t.Fatal("capacity was not bounded")
	}
	var pending []json.RawMessage
	for {
		select {
		case <-ctx.Done():
			t.Fatal("native offer did not arrive")
		case e := <-events:
			if e.Type != "signal" {
				continue
			}
			var value Signal
			if err := json.Unmarshal(e.Data, &value); err != nil {
				t.Fatal(err)
			}
			if value.Description == nil {
				pending = append(pending, value.Candidate)
				continue
			}
			if err := peer.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: value.Description.SDP}); err != nil {
				t.Fatal(err)
			}
			goto answer
		}
	}
answer:
	addCandidate := func(raw json.RawMessage) {
		if len(raw) == 0 || string(raw) == "null" {
			return
		}
		var c webrtc.ICECandidateInit
		if err := json.Unmarshal(raw, &c); err != nil {
			t.Error(err)
			return
		}
		if err := peer.AddICECandidate(c); err != nil {
			t.Error(err)
		}
	}
	for _, candidate := range pending {
		addCandidate(candidate)
	}
	description, err := peer.CreateAnswer(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := peer.SetLocalDescription(description); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(map[string]any{"description": peer.LocalDescription()})
	if err != nil {
		t.Fatal(err)
	}
	if err := host.Dispatch(ctx, Envelope{Type: "signal", SessionID: "fixture_native", Data: encoded}); err != nil {
		t.Fatal(err)
	}
	video, audio := 0, 0
	for video < 3 || audio < 5 {
		select {
		case <-ctx.Done():
			t.Fatalf("media missing: video=%d audio=%d", video, audio)
		case kind := <-packets:
			if kind == "video" {
				video++
			} else if kind == "audio" {
				audio++
			}
		case e := <-events:
			if e.Type == "error" {
				t.Fatalf("native host failed: %s", e.Data)
			}
			if e.Type == "signal" {
				var value Signal
				if err := json.Unmarshal(e.Data, &value); err != nil {
					t.Fatal(err)
				}
				addCandidate(value.Candidate)
			}
		}
	}
	stats, err := host.Stats("fixture_native")
	if err != nil {
		t.Fatal(err)
	}
	if stats["encoded_frames"].(uint64) == 0 || stats["audio_frames"].(uint64) == 0 {
		t.Fatal("native encoding counters did not advance")
	}
	snapshot, err := host.Snapshot("fixture_native")
	if err != nil || snapshot.Bounds() != image.Rect(0, 0, 720, 960) {
		t.Fatalf("missing source inspection image: %v", err)
	}
	if err := host.Dispatch(ctx, Envelope{Type: "stop", SessionID: "fixture_native"}); err != nil {
		t.Fatal(err)
	}
	if host.Active() != 0 {
		t.Fatal("stopped native session retained capacity")
	}
}
