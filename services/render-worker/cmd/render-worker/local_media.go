package main

import (
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/pion/webrtc/v4"
)

// Docker publishes this UDP port on 127.0.0.1 only. It forwards encrypted ICE,
// DTLS and SRTP, never the private worker control API or character resources.
// The caller owns the mux and closes it after every media host has stopped.
func localMediaNetwork(port int) (*webrtc.SettingEngine, io.Closer, error) {
	if port == 0 {
		return nil, nil, nil
	}
	if port < 1024 || port > 65535 {
		return nil, nil, errors.New("local media port must be zero or between 1024 and 65535")
	}
	listener, err := net.ListenUDP("udp4", &net.UDPAddr{Port: port})
	if err != nil {
		return nil, nil, fmt.Errorf("listen for local media: %w", err)
	}
	mux := webrtc.NewICEUDPMux(nil, listener)
	settings := &webrtc.SettingEngine{}
	settings.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})
	settings.SetICEUDPMux(mux)
	settings.SetNAT1To1IPs([]string{"127.0.0.1"}, webrtc.ICECandidateTypeHost)
	return settings, mux, nil
}
