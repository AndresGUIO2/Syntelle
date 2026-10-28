package nativemedia

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/pion/interceptor"
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"strings"
	"syntelle/render-contract"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
)

func (s *session) offer() error {
	s.offerMu.Lock()
	defer s.offerMu.Unlock()
	if err := s.ctx.Err(); err != nil {
		return err
	}
	engine := &webrtc.MediaEngine{}
	level := "42001f"
	if s.host.config.Width == 1440 {
		level = "420028"
	}
	if err := engine.RegisterCodec(webrtc.RTPCodecParameters{RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000, SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=" + level, RTCPFeedback: []webrtc.RTCPFeedback{{Type: "nack"}, {Type: "nack", Parameter: "pli"}, {Type: "ccm", Parameter: "fir"}}}, PayloadType: 102}, webrtc.RTPCodecTypeVideo); err != nil {
		return fmt.Errorf("register H264: %w", err)
	}
	if err := engine.RegisterCodec(webrtc.RTPCodecParameters{RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2, SDPFmtpLine: "minptime=10;useinbandfec=1"}, PayloadType: 111}, webrtc.RTPCodecTypeAudio); err != nil {
		return fmt.Errorf("register Opus: %w", err)
	}
	// No public STUN/TURN server is contacted by this local candidate.
	registry := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(engine, registry); err != nil {
		return err
	}
	options := []func(*webrtc.API){webrtc.WithMediaEngine(engine), webrtc.WithInterceptorRegistry(registry)}
	if s.host.config.NetworkSettings != nil {
		options = append(options, webrtc.WithSettingEngine(*s.host.config.NetworkSettings))
	}
	ice, err := rendercontract.ICEServers()
	if err != nil {
		return err
	}
	servers := []webrtc.ICEServer{}
	for _, server := range ice {
		servers = append(servers, webrtc.ICEServer{URLs: server.URLs, Username: server.Username, Credential: server.Credential})
	}
	pc, err := webrtc.NewAPI(options...).NewPeerConnection(webrtc.Configuration{ICEServers: servers})
	if err != nil {
		return fmt.Errorf("create native peer: %w", err)
	}
	video, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000}, "video", s.id)
	if err != nil {
		if closeErr := pc.Close(); closeErr != nil {
			return errors.Join(err, closeErr)
		}
		return err
	}
	audio, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2}, "audio", s.id)
	if err != nil {
		if closeErr := pc.Close(); closeErr != nil {
			return errors.Join(err, closeErr)
		}
		return err
	}
	videoSender, err := pc.AddTrack(video)
	if err != nil {
		if closeErr := pc.Close(); closeErr != nil {
			return errors.Join(err, closeErr)
		}
		return err
	}
	audioSender, err := pc.AddTrack(audio)
	if err != nil {
		if closeErr := pc.Close(); closeErr != nil {
			return errors.Join(err, closeErr)
		}
		return err
	}
	s.peerMu.Lock()
	previous := s.peer
	s.peer = pc
	s.videoTrack = video
	s.audioTrack = audio
	s.candidates = nil
	var bases [8]byte
	if _, err := rand.Read(bases[:]); err != nil {
		s.peerMu.Unlock()
		closeErr := pc.Close()
		return errors.Join(err, closeErr)
	}
	s.videoBase = binary.LittleEndian.Uint32(bases[:4])
	s.audioBase = binary.LittleEndian.Uint32(bases[4:])
	s.videoPackets = rtp.NewPacketizer(1200, 102, 0, &codecs.H264Payloader{}, rtp.NewRandomSequencer(), 90000)
	s.audioPackets = rtp.NewPacketizer(1200, 111, 0, &codecs.OpusPayloader{}, rtp.NewRandomSequencer(), 48000)
	s.connected.Store(false)
	s.peerMu.Unlock()
	if previous != nil {
		if err := previous.Close(); err != nil {
			return fmt.Errorf("close preceding native peer: %w", err)
		}
	}
	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if s.ctx.Err() != nil {
			return
		}
		s.peerMu.Lock()
		current := s.peer == pc
		s.peerMu.Unlock()
		if !current {
			return
		}
		var value any
		if candidate != nil {
			value = candidate.ToJSON()
		}
		s.host.emit(event("signal", s.id, "", map[string]any{"candidate": value}))
	})
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		s.peerMu.Lock()
		current := s.peer == pc
		s.peerMu.Unlock()
		if !current {
			return
		}
		s.connected.Store(state == webrtc.PeerConnectionStateConnected)
		if state == webrtc.PeerConnectionStateConnected {
			s.keyframe.Store(true)
		}
		if state == webrtc.PeerConnectionStateFailed {
			s.fail(errors.New("native media peer failed"))
		}
	})
	for _, sender := range []*webrtc.RTPSender{videoSender, audioSender} {
		s.wg.Add(1)
		go func(sender *webrtc.RTPSender) {
			defer s.wg.Done()
			for {
				packets, _, err := sender.ReadRTCP()
				if err != nil {
					return
				}
				for _, packet := range packets {
					switch packet.(type) {
					case *rtcp.PictureLossIndication, *rtcp.FullIntraRequest:
						s.keyframe.Store(true)
					}
				}
			}
		}(sender)
	}
	description, err := pc.CreateOffer(nil)
	if err != nil {
		return fmt.Errorf("create native offer: %w", err)
	}
	if err = pc.SetLocalDescription(description); err != nil {
		return fmt.Errorf("set native offer: %w", err)
	}
	s.host.emit(event("signal", s.id, "", map[string]any{"description": pc.LocalDescription()}))
	return nil
}

func (s *session) signal(raw json.RawMessage) error {
	var value Signal
	if err := json.Unmarshal(raw, &value); err != nil {
		return errors.New("invalid private media signal")
	}
	s.offerMu.Lock()
	defer s.offerMu.Unlock()
	s.peerMu.Lock()
	pc := s.peer
	s.peerMu.Unlock()
	if pc == nil || s.ctx.Err() != nil {
		return nil
	}
	if value.Description != nil {
		if value.Description.Type != "answer" || len(value.Description.SDP) > 128<<10 {
			return errors.New("expected a bounded native media answer")
		}
		if err := pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: value.Description.SDP}); err != nil {
			return fmt.Errorf("set native media answer: %w", err)
		}
		for _, candidate := range s.candidates {
			if err := pc.AddICECandidate(candidate); err != nil {
				return fmt.Errorf("add buffered native candidate: %w", err)
			}
		}
		s.candidates = nil
	}
	if len(value.Candidate) > 0 && string(value.Candidate) != "null" {
		var candidate webrtc.ICECandidateInit
		if err := json.Unmarshal(value.Candidate, &candidate); err != nil || len(candidate.Candidate) > 4096 || strings.ContainsAny(candidate.Candidate, "\r\n") {
			return errors.New("invalid native ICE candidate")
		}
		if pc.RemoteDescription() == nil {
			if len(s.candidates) >= 64 {
				return errors.New("too many buffered native candidates")
			}
			s.candidates = append(s.candidates, candidate)
		} else if err := pc.AddICECandidate(candidate); err != nil {
			return fmt.Errorf("add native candidate: %w", err)
		}
	}
	return nil
}

func (s *session) closePeer() {
	s.offerMu.Lock()
	defer s.offerMu.Unlock()
	s.connected.Store(false)
	s.peerMu.Lock()
	pc := s.peer
	s.peer = nil
	s.videoTrack = nil
	s.audioTrack = nil
	s.peerMu.Unlock()
	if pc != nil {
		if err := pc.Close(); err != nil {
			s.host.emit(event("error", s.id, "", map[string]string{"message": "The private media peer could not close."}))
		}
	}
}

// sendEncoded linearizes interruption against final RTP publication. Timestamps
// come from the media epoch, so a missed video tick cannot slow the audio clock.
func (s *session) sendEncoded(data []byte, video bool, pts time.Duration, generation uint64) (bool, error) {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	return s.sendEncodedLocked(data, video, pts, generation)
}

// Caller owns sendMu across first-PCM selection, publication and policy commit.
func (s *session) sendEncodedLocked(data []byte, video bool, pts time.Duration, generation uint64) (bool, error) {
	s.mu.Lock()
	current := generation == s.generation
	s.mu.Unlock()
	if !current || s.ctx.Err() != nil {
		return false, nil
	}
	s.peerMu.Lock()
	defer s.peerMu.Unlock()
	if !s.connected.Load() {
		return false, nil
	}
	track, packetizer, clock, base := s.audioTrack, s.audioPackets, uint64(48000), s.audioBase
	if video {
		track = s.videoTrack
		packetizer = s.videoPackets
		clock = 90000
		base = s.videoBase
	}
	if track == nil || packetizer == nil {
		return false, nil
	}
	timestamp := base + uint32(uint64(pts/time.Second)*clock+uint64(pts%time.Second)*clock/uint64(time.Second))
	for _, packet := range packetizer.Packetize(data, 0) {
		packet.Timestamp = timestamp
		if err := track.WriteRTP(packet); err != nil {
			return false, err
		}
	}
	if video {
		s.videoBytes.Add(uint64(len(data)))
	} else {
		s.audioBytes.Add(uint64(len(data)))
	}
	return true, nil
}
