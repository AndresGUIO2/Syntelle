package nativemedia

import (
	"github.com/pion/interceptor"
	"github.com/pion/rtcp"
	"sync"
	"time"
)

// Only clock mappings are replaced. Default report counters/feedback stay intact.
type mediaEpochReports struct {
	mu           sync.Mutex
	epoch        time.Time
	audio, video uint32
	streams      map[uint32]bool
}

func ntpTime(t time.Time) uint64 {
	return uint64(t.Unix()+2208988800)<<32 | uint64(t.Nanosecond())*(1<<32)/1000000000
}
func (c *mediaEpochReports) start(now time.Time, pts time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.epoch.IsZero() {
		c.epoch = now.Add(-pts)
	}
}
func (c *mediaEpochReports) rewrite(packets []rtcp.Packet) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.epoch.IsZero() {
		return
	}
	epoch := ntpTime(c.epoch)
	for _, p := range packets {
		if sr, ok := p.(*rtcp.SenderReport); ok {
			video, known := c.streams[sr.SSRC]
			if !known {
				continue
			}
			base, rate := c.audio, uint64(48000)
			if video {
				base, rate = c.video, 90000
			}
			delta := sr.NTPTime - epoch
			sr.RTPTime = base + uint32((delta>>32)*rate+((delta&0xffffffff)*rate>>32))
		}
	}
}

type mediaEpochFactory struct{ clock *mediaEpochReports }

func (f mediaEpochFactory) NewInterceptor(string) (interceptor.Interceptor, error) {
	return &mediaEpochInterceptor{clock: f.clock}, nil
}

type mediaEpochInterceptor struct {
	interceptor.NoOp
	clock *mediaEpochReports
}

func (i *mediaEpochInterceptor) BindLocalStream(info *interceptor.StreamInfo, writer interceptor.RTPWriter) interceptor.RTPWriter {
	i.clock.mu.Lock()
	if i.clock.streams == nil {
		i.clock.streams = map[uint32]bool{}
	}
	i.clock.streams[info.SSRC] = info.MimeType == "video/H264"
	i.clock.mu.Unlock()
	return writer
}
func (i *mediaEpochInterceptor) BindRTCPWriter(writer interceptor.RTCPWriter) interceptor.RTCPWriter {
	return interceptor.RTCPWriterFunc(func(p []rtcp.Packet, a interceptor.Attributes) (int, error) {
		i.clock.rewrite(p)
		return writer.Write(p, a)
	})
}
