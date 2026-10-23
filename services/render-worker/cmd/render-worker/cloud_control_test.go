package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"syntelle/render-contract"
	"syntelle/render-worker/internal/nativemedia"
)

type unusedEncoders struct{}

func (unusedEncoders) Video(int, int, int, int) (nativemedia.VideoEncoder, error) {
	return nil, errors.New("offline fixture never encodes")
}
func (unusedEncoders) Audio(int, int, int) (nativemedia.AudioEncoder, error) {
	return nil, errors.New("offline fixture never encodes")
}

func TestCloudControlAuthenticatesAndReturnsOnDisconnect(t *testing.T) {
	t.Setenv("SYNTELLE_DEPLOYMENT_MODE", "cloud")
	t.Setenv("SYNTELLE_RENDER_ALLOCATION_ID", "allocation_fixture")
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Header.Get("Authorization") != "Bearer offline-token" || r.Header.Get("X-Syntelle-Allocation") != "allocation_fixture" {
			t.Error("missing private worker registration credentials")
			w.WriteHeader(403)
			return
		}
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer c.CloseNow()
		_ = wsjson.Write(r.Context(), c, nativemedia.Envelope{Type: "registered"})
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	err := connection(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), "offline-token", nativemedia.Config{
		PackageHash: strings.Repeat("a", 64), Encoders: unusedEncoders{},
		SceneFactory: func(rendercontract.Settings) (nativemedia.Scene, error) {
			return nil, errors.New("offline fixture never renders")
		},
	})
	if err == nil || ctx.Err() != nil || requests.Load() != 1 {
		t.Fatal("control disconnect did not promptly end the cloud connection", err)
	}
}
