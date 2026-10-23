// Command render-probe is an offline Linux measurement fixture. Publish only
// on host loopback in explicit --loopback-docker mode; it accepts fixture control.
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image/png"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"slices"
	"sync"
	"syscall"
	"time"

	"github.com/pion/webrtc/v4"
	"syntelle/render-worker/internal/nativeadapter"
	"syntelle/render-worker/internal/nativeclock"
	"syntelle/render-worker/internal/nativemedia"
)

type probe struct {
	mu     sync.Mutex
	host   *nativemedia.Host
	cursor int
	events []map[string]any
	ids    []string
}

func (p *probe) emit(e nativemedia.Envelope) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cursor++
	p.events = append(p.events, map[string]any{"cursor": p.cursor, "type": e.Type, "session_id": e.SessionID, "command_id": e.CommandID, "data": e.Data})
	if len(p.events) > 1000 {
		p.events = p.events[len(p.events)-1000:]
	}
}
func (p *probe) snapshot() map[string]any {
	p.mu.Lock()
	ids := slices.Clone(p.ids)
	p.mu.Unlock()
	pages := []any{}
	for _, id := range ids {
		if stats, err := p.host.Stats(id); err == nil {
			pages = append(pages, stats)
		}
	}
	read := func(name string) string {
		b, _ := os.ReadFile("/sys/fs/cgroup/" + name)
		return string(bytes.TrimSpace(b))
	}
	number := func(name string) uint64 { var n uint64; fmt.Sscan(read(name), &n); return n }
	return map[string]any{"raw_monotonic_ns": nativeclock.Now(), "active": p.host.Active(), "diagnostics": []string{}, "pages": pages, "cgroup": map[string]any{"cpu": read("cpu.stat"), "cpu_max": read("cpu.max"), "memory": number("memory.current"), "peak": number("memory.peak"), "memory_max": read("memory.max"), "memory_events": read("memory.events")}}
}
func (p *probe) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var value any
	var err error
	switch r.URL.Path {
	case "/health":
		value = map[string]any{"ready": true, "runtime": runtime.Version(), "renderer_configured": os.Getenv("SYNTELLE_RENDERER"), "codec": "H264/Opus"}
	case "/events":
		after := 0
		fmt.Sscan(r.URL.Query().Get("after"), &after)
		p.mu.Lock()
		out := []map[string]any{}
		for _, e := range p.events {
			if e["cursor"].(int) > after {
				out = append(out, e)
			}
		}
		p.mu.Unlock()
		value = out
	case "/snapshot":
		value = p.snapshot()
	case "/command":
		var e nativemedia.Envelope
		err = json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&e)
		if err == nil {
			err = p.host.Dispatch(r.Context(), e)
		}
		if err == nil {
			p.mu.Lock()
			if e.Type == "create" && !slices.Contains(p.ids, e.SessionID) {
				p.ids = append(p.ids, e.SessionID)
			} else if e.Type == "stop" {
				p.ids = slices.DeleteFunc(p.ids, func(id string) bool { return id == e.SessionID })
			}
			p.mu.Unlock()
			value = map[string]any{"accepted": true}
		}
	case "/timing":
		var input struct {
			SessionID string `json:"session_id"`
		}
		err = json.NewDecoder(io.LimitReader(r.Body, 1024)).Decode(&input)
		if err == nil {
			err = p.host.MeasureTiming(input.SessionID)
		}
		value = map[string]bool{"accepted": err == nil}
	case "/source":
		var input struct{ Index int }
		err = json.NewDecoder(io.LimitReader(r.Body, 1024)).Decode(&input)
		if err == nil {
			p.mu.Lock()
			id := ""
			if input.Index >= 0 && input.Index < len(p.ids) {
				id = p.ids[input.Index]
			}
			p.mu.Unlock()
			if id == "" {
				err = errors.New("no probe session")
			} else {
				frame, e := p.host.Snapshot(id)
				err = e
				if err == nil {
					var buffer bytes.Buffer
					err = png.Encode(&buffer, frame)
					value = "data:image/png;base64," + base64.StdEncoding.EncodeToString(buffer.Bytes())
				}
			}
		}
	default:
		w.WriteHeader(404)
		value = map[string]any{"error": "unknown probe route"}
	}
	if err != nil {
		w.WriteHeader(500)
		value = map[string]any{"error": err.Error()}
	}
	_ = json.NewEncoder(w).Encode(value)
}
func run(ctx context.Context) error {
	assets := flag.String("assets", "", "private native resource directory")
	listen := flag.String("listen", "127.0.0.1:3000", "offline probe address")
	unpublished := flag.Bool("unpublished-network", false, "permit an unpublished dedicated-container network")
	foreground := flag.Bool("foreground", false, "replay the current packed character transport")
	loopbackDocker := flag.Bool("loopback-docker", false, "explicit local fixture; publish HTTP and media on host 127.0.0.1 only")
	mediaPort := flag.Int("local-media-port", 0, "loopback-only Docker fixture UDP port")
	sessions := flag.Int("sessions", 1, "bounded offline fixture capacity: 1, 2 or 4")
	flag.Parse()
	if *sessions != 1 && *sessions != 2 && *sessions != 4 {
		return errors.New("offline probe supports 1, 2 or 4 sessions")
	}
	if *listen != "127.0.0.1:3000" && ((!*unpublished && !*loopbackDocker) || *listen != "0.0.0.0:3000") {
		return errors.New("probe must be loopback or an explicitly unpublished fixture network")
	}
	var network *webrtc.SettingEngine
	if *loopbackDocker {
		if *mediaPort < 1024 || *mediaPort > 65535 {
			return errors.New("explicit loopback UDP fixture port required")
		}
		listener, err := net.ListenUDP("udp4", &net.UDPAddr{Port: *mediaPort})
		if err != nil {
			return err
		}
		mux := webrtc.NewICEUDPMux(nil, listener)
		defer mux.Close()
		network = &webrtc.SettingEngine{}
		network.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})
		network.SetICEUDPMux(mux)
		network.SetNAT1To1IPs([]string{"127.0.0.1"}, webrtc.ICECandidateTypeHost)
	} else if *mediaPort != 0 {
		return errors.New("media mapping requires explicit loopback Docker mode")
	}
	load := nativeadapter.Load
	width := 720
	if *foreground {
		load = nativeadapter.LoadForeground
		width = 1440
	}
	factory, hash, err := load(*assets)
	if err != nil {
		return err
	}
	p := &probe{}
	p.host, err = nativemedia.New(ctx, nativemedia.Config{Capacity: *sessions, Width: width, PackageHash: hash, SceneFactory: factory, Emit: p.emit, OfflineCalibration: true, NetworkSettings: network})
	if err != nil {
		return err
	}
	defer p.host.Close()
	server := &http.Server{Addr: *listen, Handler: p, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second}
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	select {
	case err = <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		stop, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(stop)
		<-done
	}
	return nil
}
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
