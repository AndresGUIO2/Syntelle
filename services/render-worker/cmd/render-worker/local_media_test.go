package main

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"
)

func TestLocalMediaNetworkConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name    string
		port    int
		invalid bool
	}{
		{"disabled", 0, false}, {"negative", -1, true},
		{"privileged", 80, true}, {"overflow", 65536, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings, closer, err := localMediaNetwork(tc.port)
			if (err != nil) != tc.invalid || settings != nil || closer != nil {
				t.Fatalf("unexpected configuration result: settings=%v closer=%v error=%v", settings, closer, err)
			}
		})
	}
}

func TestLocalMediaNetworkSharedPortAndRelease(t *testing.T) {
	reservation, err := net.ListenUDP("udp4", &net.UDPAddr{})
	if err != nil {
		t.Fatal(err)
	}
	port := reservation.LocalAddr().(*net.UDPAddr).Port
	if _, _, err := localMediaNetwork(port); err == nil {
		reservation.Close()
		t.Fatal("occupied port must fail rather than select a different port")
	}
	if err := reservation.Close(); err != nil {
		t.Fatal(err)
	}
	settings, closer, err := localMediaNetwork(port)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := closer.Close(); err != nil {
			t.Error(err)
		}
	})
	for range 2 {
		peer, err := webrtc.NewAPI(webrtc.WithSettingEngine(*settings)).NewPeerConnection(webrtc.Configuration{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := peer.CreateDataChannel("network-test", nil); err != nil {
			peer.Close()
			t.Fatal(err)
		}
		complete := webrtc.GatheringCompletePromise(peer)
		offer, err := peer.CreateOffer(nil)
		if err != nil {
			peer.Close()
			t.Fatal(err)
		}
		if err := peer.SetLocalDescription(offer); err != nil {
			peer.Close()
			t.Fatal(err)
		}
		select {
		case <-complete:
		case <-time.After(3 * time.Second):
			peer.Close()
			t.Fatal("candidate gathering timed out")
		}
		if !strings.Contains(peer.LocalDescription().SDP, fmt.Sprintf(" 127.0.0.1 %d typ host", port)) {
			peer.Close()
			t.Fatal("peer did not advertise the Docker loopback media mapping")
		}
		if err := peer.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if err := closer.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := net.ListenUDP("udp4", &net.UDPAddr{Port: port})
	if err != nil {
		t.Fatalf("media shutdown did not release UDP socket: %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
}
