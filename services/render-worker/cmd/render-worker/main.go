package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"syntelle/render-worker/internal/nativemedia"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("Native render worker stopped", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) (runErr error) {
	capacity, err := strconv.Atoi(env("SYNTELLE_RENDER_CAPACITY", "1"))
	if err != nil || capacity < 1 || capacity > 64 {
		return errors.New("configure a render capacity between one and 64")
	}
	assets := flag.String("assets", os.Getenv("SYNTELLE_NATIVE_ASSETS"), "private prepared native asset directory")
	endpoint := flag.String("api", env("SYNTELLE_RENDER_API_URL", "http://127.0.0.1:8080"), "local owner API URL")
	limit := flag.Int("capacity", capacity, "bounded native media capacity")
	mediaPort := flag.Int("local-media-port", 0, "Docker loopback-published UDP media port; zero keeps direct ICE")
	flag.Parse()
	if *assets == "" {
		return errors.New("configure private native character assets")
	}
	address, err := url.Parse(*endpoint)
	if err != nil || address.User != nil || (address.Scheme != "http" && address.Scheme != "https") || (address.Hostname() != "127.0.0.1" && address.Hostname() != "localhost" && address.Hostname() != "::1") {
		return errors.New("native render worker requires a loopback API URL")
	}
	if address.Scheme == "https" {
		address.Scheme = "wss"
	} else {
		address.Scheme = "ws"
	}
	address.Path = "/api/v1/stream-worker/ws"
	address.RawQuery = ""
	address.Fragment = ""
	token := os.Getenv("SYNTELLE_RENDER_WORKER_TOKEN")
	if len(token) < 32 {
		return errors.New("configure the private render worker token")
	}
	network, media, err := localMediaNetwork(*mediaPort)
	if err != nil {
		return err
	}
	if media != nil {
		defer func() { runErr = errors.Join(runErr, media.Close()) }()
	}
	factory, hash, err := loadSceneFactory(*assets)
	if err != nil {
		return fmt.Errorf("load private native character: %w", err)
	}
	backoff := time.Second
	for ctx.Err() == nil {
		err = connection(ctx, address.String(), token, nativemedia.Config{Capacity: *limit, Width: 1440, Height: 960, FPS: 30, PackageHash: hash, SceneFactory: factory, NetworkSettings: network})
		if ctx.Err() != nil {
			return ctx.Err()
		}
		slog.Warn("Native render control disconnected", "error", err)
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		backoff = min(backoff*2, 10*time.Second)
	}
	return ctx.Err()
}

func connection(parent context.Context, address, token string, config nativemedia.Config) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	dial, stop := context.WithTimeout(ctx, 10*time.Second)
	c, _, err := websocket.Dial(dial, address, &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer " + token}}})
	stop()
	if err != nil {
		return fmt.Errorf("connect private control: %w", err)
	}
	defer c.CloseNow()
	c.SetReadLimit(16 << 20)
	out := make(chan nativemedia.Envelope, 32)
	config.Emit = func(value nativemedia.Envelope) {
		select {
		case out <- value:
		case <-ctx.Done():
		default:
			slog.Warn("Native control queue full", "event_type", value.Type, "queued", len(out))
			cancel()
		}
	}
	host, err := nativemedia.New(ctx, config)
	if err != nil {
		return err
	}
	defer host.Close()
	written := make(chan error, 1)
	go func() {
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				written <- ctx.Err()
				return
			case value := <-out:
				write, done := context.WithTimeout(ctx, 5*time.Second)
				err := wsjson.Write(write, c, value)
				done()
				if err != nil {
					slog.Warn("Native control write failed", "event_type", value.Type, "error_class", controlErrorClass(err))
					written <- err
					return
				}
			}
		}
	}()
	defer func() { cancel(); <-written }()
	for {
		var command nativemedia.Envelope
		if err := wsjson.Read(ctx, c, &command); err != nil {
			return err
		}
		if command.Type == "registered" {
			slog.Info("Native render worker connected")
			continue
		}
		if err := host.Dispatch(ctx, command); err != nil {
			slog.Warn("Native render command failed", "kind", command.Type, "error", err)
			config.Emit(nativemedia.Envelope{Type: "error", SessionID: command.SessionID, Data: []byte(`{"message":"The private native renderer could not process this command."}`)})
		}
	}
}

// Keep private transport diagnostics useful without logging command bodies,
// tickets, SDP, candidates or authentication material.
func controlErrorClass(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline"
	case websocket.CloseStatus(err) != -1:
		return "websocket_closed"
	default:
		return "transport"
	}
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
