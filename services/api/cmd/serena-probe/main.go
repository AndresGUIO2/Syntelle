// Bounded owner-only qualification; deliberately absent from HTTP routes.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/jackc/pgx/v5"
	"os"
	"strings"
	"syntelle/api/internal/conversation"
	"syntelle/api/internal/speech"
	"syntelle/render-contract"
	"time"
	"unicode/utf16"
)

const identity = "authorized-coordinated-serena-2026-10-27"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	slot := flag.String("slot", "", "fixed purpose-language-mode or laughter-language")
	catalogPath := flag.String("catalog", "", "regional availability receipt")
	flag.Parse()
	region := os.Getenv("AZURE_SPEECH_REGION")
	if region != "canadacentral" {
		return errors.New("use the selected personal Canada Central resource")
	}
	var catalog struct {
		Region, Voice, Status string
		AvailableLanguages    []string `json:"probe_languages"`
	}
	data, err := os.ReadFile(*catalogPath)
	if err != nil || json.Unmarshal(data, &catalog) != nil || catalog.Status != "available" || catalog.Region != region || catalog.Voice != "en-US-Serena:DragonHDLatestNeural" {
		return errors.New("verified regional Serena catalog required before synthesis")
	}
	parts := strings.Split(*slot, "-")
	if len(parts) < 2 {
		return errors.New("choose a fixed qualification slot")
	}
	language := "en-US"
	if parts[1] == "es" {
		language = "es-CO"
	} else if parts[1] != "en" {
		return errors.New("unknown probe language")
	}
	available := false
	for _, locale := range catalog.AvailableLanguages {
		available = available || locale == language
	}
	if !available {
		return errors.New("Serena language is not available in the regional catalog")
	}
	texts := map[string][2]string{
		"reassure": {"I can help you take this one step at a time.", "Podemos resolverlo paso a paso; estoy aquí para ayudarte."},
		"ask":      {"What would you like to try first?", "¿Qué te gustaría probar primero?"},
		"playful":  {"All right, you caught me. That was a good one!", "Está bien, me descubriste. ¡Esa estuvo buena!"},
	}
	text := "[laughter]"
	style := ""
	if parts[0] == "laughter" {
		if len(parts) != 2 {
			return errors.New("invalid laughter slot")
		}
	} else {
		pair, ok := texts[parts[0]]
		if !ok || len(parts) != 3 || (parts[2] != "plain" && parts[2] != "directed") {
			return errors.New("invalid fixed speech slot")
		}
		text = pair[0]
		if language == "es-CO" {
			text = pair[1]
		}
		if parts[2] == "directed" {
			if language != "en-US" {
				return errors.New("Spanish explicit styles remain unsupported; slot skipped before any request")
			}
			style = map[string]string{"reassure": "reassuring", "ask": "curious", "playful": "amused"}[parts[0]]
		}
	}
	input := speech.Input{Text: text, Voice: "en-US-Serena:DragonHDLatestNeural", Language: language, RequestKey: identity + "-" + *slot}
	if style != "" {
		input, err = speech.WithVoiceDelivery(input, rendercontract.VoiceDelivery{Version: rendercontract.DeliveryVersion, Purpose: parts[0], Emotion: rendercontract.EmotionTarget{State: "neutral", Energy: .5}, Style: style, CapabilityRevision: identity + "-qualification"})
		if err != nil {
			return err
		}
	}
	characters := len(utf16.Encode([]rune(input.ProviderInput())))
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()
	store, err := conversation.Open(ctx, os.Getenv("SYNTELLE_DATABASE_URL"))
	if err != nil {
		return err
	}
	defer store.Pool.Close()
	guard, err := store.Pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer guard.Release()
	if _, err = guard.Exec(ctx, "SELECT pg_advisory_lock(187200103)"); err != nil {
		return err
	}
	defer func() {
		c, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		if _, e := guard.Exec(c, "SELECT pg_advisory_unlock(187200103)"); e != nil {
			_ = guard.Hijack().Close(c)
		}
	}()
	request := map[string]any{"input": input, "region": region, "characters": characters, "serializer": "azure-ssml/1", "sdk": "1.52.0", "format": "riff-24khz-16bit-mono-pcm", "catalog": json.RawMessage(data), "limits": map[string]int{"tts": 14, "characters": 4000, "image": 0, "analysis": 0}, "retries": 0}
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}
	binding := fmt.Sprintf("%x", sha256.Sum256(body))
	var oldStatus string
	var oldResult []byte
	var oldBinding string
	err = guard.QueryRow(ctx, "SELECT status,result,binding FROM syntelle_conversation.operations WHERE kind=$1 AND request_key=$2", identity, *slot).Scan(&oldStatus, &oldResult, &oldBinding)
	if err == nil {
		if oldStatus != "received" || oldBinding != binding {
			return errors.New("existing failed or uncertain slot cannot be retried")
		}
		_, err = os.Stdout.Write(append(oldResult, '\n'))
		return err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return errors.New("qualification history unavailable")
	}
	var count, total int
	var halted bool
	if err = guard.QueryRow(ctx, "SELECT count(*),coalesce(sum((request->>'characters')::int),0),coalesce(bool_or(status<>'received' AND coalesce(result->>'provider_status','')<>'SynthesizingAudioCompleted'),false) FROM syntelle_conversation.operations WHERE kind=$1", identity).Scan(&count, &total, &halted); err != nil {
		return errors.New("qualification budget unavailable")
	}
	if halted || count >= 14 || total+characters > 4000 {
		return errors.New("qualification halted or bounded allowance exhausted")
	}

	op, err := store.Reserve(ctx, identity, *slot, binding, "", "", request, false)
	if err != nil {
		return err
	}
	provider, err := speech.NewAzureProvider(os.Getenv("AZURE_SPEECH_KEY"), region)
	var result speech.Result
	if err == nil {
		result, err = provider.Synthesize(ctx, input)
	}
	if err == nil && parts[0] != "laughter" {
		err = speech.ValidateResult(result)
	} else if err == nil {
		err = speech.ValidateLaughterResult(result)
	}
	result.Region = region
	status := "received"
	if err != nil {
		status = "failed"
	}
	finishErr := store.Finish(ctx, op.ID, status, result, nil, map[string]any{"no_retry": true, "slot": *slot, "provider_error": err != nil}, result.Audio)
	if finishErr != nil {
		return errors.New("qualification evidence commit failed; no retry")
	}
	if err != nil {
		return errors.New("Serena provider failed; qualification is halted without retries")
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}
