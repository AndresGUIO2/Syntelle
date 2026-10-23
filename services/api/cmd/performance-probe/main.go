// A bounded owner experiment, deliberately absent from the HTTP service.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"syntelle/api/internal/conversation"
	"syntelle/api/internal/speech"
)

const identity = "authorized-character-performance-2026-10-20"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	slot := flag.String("slot", "", "gemini-en, gemini-es, tts-en or tts-es")
	flag.Parse()
	language, prompt, kind := "en-US", "", "gemini"
	switch *slot {
	case "gemini-en":
		prompt = "In English, answer these as a single brief conversational reply: can we keep a steady posture, does the word no always mean disagreement, and show one light playful example? Use directions only when they make semantic sense."
	case "gemini-es":
		language = "es-CO"
		prompt = "En español, responde brevemente y con seriedad: ¿es seguro ignorar un aviso de incendio? No añadas humor."
	case "tts-en":
		kind = "tts"
		prompt = "[laughter]"
	case "tts-es":
		kind = "tts"
		language = "es-CO"
		prompt = "[laughter]"
	default:
		return errors.New("choose one of the four fixed authorized probe slots")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	store, err := conversation.Open(ctx, os.Getenv("SYNTELLE_DATABASE_URL"))
	if err != nil {
		return err
	}
	defer store.Pool.Close()
	// One session lock serializes the complete experiment, including network work.
	guard, err := store.Pool.Acquire(ctx)
	if err != nil {
		return errors.New("probe ledger unavailable")
	}
	defer guard.Release()
	if _, err = guard.Exec(ctx, "SELECT pg_advisory_lock(187200034)"); err != nil {
		return errors.New("probe lock unavailable")
	}
	defer func() {
		unlock, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		if _, e := guard.Exec(unlock, "SELECT pg_advisory_unlock(187200034)"); e != nil {
			guard.Hijack().Close(unlock)
		}
	}()
	var stopped bool
	if err = guard.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM syntelle_conversation.operations WHERE kind=$1 AND status<>'received')`, identity).Scan(&stopped); err != nil {
		return errors.New("probe ledger check failed")
	}
	if stopped {
		return errors.New("experiment halted by an earlier failed or uncertain probe; no further paid work")
	}
	if err = guard.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM syntelle_preparation.budgets WHERE id=$1 AND halted) OR EXISTS(SELECT 1 FROM syntelle_preparation.requests WHERE budget_id=$1 AND status<>'received')`, identity).Scan(&stopped); err != nil {
		return errors.New("image experiment ledger unavailable; no paid work")
	}
	if stopped {
		return errors.New("image failure or uncertain request halted all paid work")
	}
	gemini, err := conversation.NewGemini(os.Getenv("GEMINI_API_KEY"), conversation.GeminiChatModel)
	if err != nil {
		return err
	}
	gemini.WithDirection()
	messages := []conversation.Message{{Role: "user", Content: prompt}}
	input := speech.Input{Text: prompt, Language: language, Voice: "en-US-Ava:DragonHDLatestNeural", RequestKey: identity + "-" + *slot}
	var request any = map[string]any{"input": input, "region": os.Getenv("AZURE_SPEECH_REGION"), "format": "riff-24khz-16bit-mono-pcm", "sdk": "1.52.0"}
	if kind == "gemini" {
		request = gemini.Request("Selected language: "+language, messages)
	}
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(body)
	operation, err := store.Reserve(ctx, identity, *slot, hex.EncodeToString(hash[:]), "", "", map[string]any{"request": request, "ceiling": map[string]int{"image": 2, "tts": 2, "gemini": 2, "analysis": 0}, "retries": 0}, false)
	if err != nil {
		return err
	}
	if operation.Status == "received" {
		_, err = os.Stdout.Write(append(operation.Result, '\n'))
		return err
	}
	var result any
	var audio []byte
	if kind == "gemini" {
		result, err = gemini.Complete(ctx, "Selected language: "+language, messages, func(string) {})
	} else {
		var provider speech.Provider
		provider, err = speech.NewAzureProvider(os.Getenv("AZURE_SPEECH_KEY"), os.Getenv("AZURE_SPEECH_REGION"))
		if err == nil {
			var r speech.Result
			r, err = provider.Synthesize(ctx, input)
			result = r
			audio = r.Audio
		}
	}
	status := "received"
	if err != nil {
		status = "failed"
	}
	if finishErr := store.Finish(ctx, operation.ID, status, result, nil, map[string]any{"outcome": status, "no_retry": true}, audio); finishErr != nil {
		return errors.New("probe evidence commit failed; no retry")
	}
	data, encodeErr := json.Marshal(result)
	if encodeErr != nil {
		return encodeErr
	}
	if _, writeErr := os.Stdout.Write(append(data, '\n')); writeErr != nil {
		return writeErr
	}
	if err != nil {
		return fmt.Errorf("%s probe failed: %w", kind, err)
	}
	return nil
}
