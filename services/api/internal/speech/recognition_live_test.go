//go:build speech && linux

package speech

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syntelle/api/internal/conversation"
	"testing"
	"time"
)

// Explicit, live owner verification only. Never runs during normal builds.
// Inputs are previously archived TTS, not captured microphone recordings.
func TestOwnerRecognitionSmoke(t *testing.T) {
	if os.Getenv("SYNTELLE_LIVE_RECOGNITION") != "1" {
		t.Skip("Live paid recognition is explicitly opt-in")
	}
	ctx := context.Background()
	store, e := conversation.Open(ctx, os.Getenv("SYNTELLE_DATABASE_URL"))
	if e != nil {
		t.Fatal(e)
	}
	defer store.Pool.Close()
	service := conversation.New(store, nil, nil, NewAzureRecognition(os.Getenv("AZURE_SPEECH_KEY"), os.Getenv("AZURE_SPEECH_REGION")))
	for _, fixture := range []struct{ id, language string }{{"ava-en", "en-US"}, {"ava-es", "es-CO"}} {
		cid := "live-recognition-" + fixture.id + "-20261020"
		operation := "native-recognition-" + fixture.id + "-20261020"
		if _, e = store.Session(ctx, cid, "syntelle", fixture.language, "Owner-authorized native recognition verification; no model call."); e != nil {
			t.Fatal(e)
		}
		pcm, e := os.ReadFile(filepath.Join("/fixtures", fixture.id+".pcm"))
		if e != nil {
			t.Fatal(e)
		}
		work, cancel := context.WithTimeout(ctx, 20*time.Second)
		stream := make(chan []byte, 4)
		done := make(chan struct{})
		final := make(chan struct{}, 1)
		events := []conversation.RecognitionEvent{}
		var mu sync.Mutex
		started := time.Now()
		firstFinal := int64(-1)
		go func() {
			defer close(done)
			service.RunRecognition(work, cid, operation, fixture.language, stream, func(event conversation.RecognitionEvent) {
				mu.Lock()
				events = append(events, event)
				if event.Type == "final" && firstFinal < 0 {
					firstFinal = time.Since(started).Milliseconds()
				}
				mu.Unlock()
				if event.Type == "final" {
					select {
					case final <- struct{}{}:
					default:
					}
				}
			})
		}()
		for i := 0; i < len(pcm); i += 3200 {
			end := i + 3200
			if end > len(pcm) {
				end = len(pcm)
			}
			select {
			case stream <- pcm[i:end]:
			case <-work.Done():
			}
			select {
			case <-time.After(100 * time.Millisecond):
			case <-work.Done():
			}
		}
		// Final silence is recognition input only, never persisted.
		for i := 0; i < 10; i++ {
			select {
			case stream <- make([]byte, 3200):
			case <-work.Done():
			}
			time.Sleep(100 * time.Millisecond)
		}
		select {
		case <-final:
		case <-work.Done():
		case <-time.After(5 * time.Second):
		}
		cancel()
		<-done
		mu.Lock()
		report := map[string]any{"case": fixture.id, "language": fixture.language, "sdk_version": speechSDKVersion, "first_final_ms": firstFinal, "events": events, "source": "previously archived Dragon HD TTS", "new_model_or_synthesis_calls": 0}
		data, _ := json.MarshalIndent(report, "", "  ")
		mu.Unlock()
		if e = os.WriteFile(filepath.Join("/evidence", "recognition-"+fixture.id+".json"), data, 0600); e != nil {
			t.Fatal(e)
		}
		if firstFinal < 0 {
			t.Fatal("Native recognition returned no final transcript; do not retry automatically.")
		}
		fmt.Printf("Native %s recognition final at %d ms; operation recorded.\n", fixture.language, firstFinal)
	}
}
