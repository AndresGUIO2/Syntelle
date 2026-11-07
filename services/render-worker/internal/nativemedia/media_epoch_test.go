package nativemedia

import (
	"github.com/pion/rtcp"
	"testing"
	"time"
)

func TestMediaEpochIgnoresUnequalTransmissionDelay(t *testing.T) {
	c := mediaEpochReports{audio: 700, video: 900, streams: map[uint32]bool{1: false, 2: true}}
	now := time.Unix(1750000000, 0)
	c.start(now, time.Second)
	c.start(now.Add(350*time.Millisecond), time.Second)
	a := &rtcp.SenderReport{SSRC: 1, NTPTime: ntpTime(now.Add(time.Second)), RTPTime: 42, PacketCount: 11}
	v := &rtcp.SenderReport{SSRC: 2, NTPTime: a.NTPTime, RTPTime: 15, PacketCount: 7}
	c.rewrite([]rtcp.Packet{a, v})
	if a.RTPTime != 96700 || v.RTPTime != 180900 || a.PacketCount != 11 || v.PacketCount != 7 {
		t.Fatal("independent send clocks", a, v)
	}
}
