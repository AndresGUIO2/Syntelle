package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/coder/websocket"
	"net/http"
	"os"
	"syntelle/api/internal/conversation"
	"testing"
	"time"
)

// Explicit local transport verification, with no submitted turn/provider call.
func TestLocalConversationProxy(t *testing.T) {
	if os.Getenv("SYNTELLE_VERIFY_LOCAL_PROXY") != "1" {
		t.Skip("Local running-service verification is opt-in")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, e := websocket.Dial(ctx, "ws://127.0.0.1:5173/api/v1/conversation/ws", &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{"http://127.0.0.1:5173"}}})
	if e != nil {
		t.Fatal(e)
	}
	defer c.CloseNow()
	cid := fmt.Sprintf("local-transport-%d", time.Now().UnixNano())
	event := conversation.Event{Type: "hello", ConversationID: cid, TurnID: "transport-hello-20261020", Character: "syntelle", Language: "en-US", Voice: "en-US-Ava:DragonHDLatestNeural", Instructions: "Owner local WebSocket transport verification. No user turn or provider request."}
	b, _ := json.Marshal(event)
	if e = c.Write(ctx, websocket.MessageText, b); e != nil {
		t.Fatal(e)
	}
	_, b, e = c.Read(ctx)
	if e != nil {
		t.Fatal(e)
	}
	var ready conversation.Event
	if e = json.Unmarshal(b, &ready); e != nil || ready.Type != "ready" || ready.ConversationID != cid {
		t.Fatal("Proxy did not restore the session", e)
	}
	t.Log("Vite → Go → PostgreSQL session handshake passed; zero submitted turns/provider calls.")
}
