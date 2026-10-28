// Bounded private nonverbal preparation. Never exposed through HTTP.
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
	"syntelle/api/internal/conversation"
	"syntelle/api/internal/speech"
	"time"
	"unicode/utf16"
)

const identity = "owner-requested-emma-laughter-2026-10-27"
const voice = "en-US-Emma:DragonHDLatestNeural"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	language := flag.String("language", "", "en-US or es-CO")
	catalogPath := flag.String("catalog", "", "regional availability receipt")
	flag.Parse()
	text := "[laughter]"
	if (*language != "en-US" && *language != "es-CO") || os.Getenv("AZURE_SPEECH_REGION") != "canadacentral" {
		return errors.New("use the selected voice, language and personal region")
	}
	raw, err := os.ReadFile(*catalogPath)
	var catalog struct{ Voice, Region, Status string }
	if err != nil || json.Unmarshal(raw, &catalog) != nil || catalog.Voice != voice || catalog.Region != "canadacentral" || catalog.Status != "available" {
		return errors.New("regional Emma availability required")
	}
	input := speech.Input{Text: text, Voice: voice, Language: *language, RequestKey: identity + "-" + *language}
	count := len(utf16.Encode([]rune(text)))
	request := map[string]any{"input": input, "catalog": json.RawMessage(raw), "region": "canadacentral", "sdk": "1.52.0", "format": "riff-24khz-16bit-mono-pcm", "characters": count, "limits": map[string]int{"attempts": 2, "characters": 20}, "retries": 0}
	data, _ := json.Marshal(request)
	binding := fmt.Sprintf("%x", sha256.Sum256(data))
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
	if _, err = guard.Exec(ctx, "SELECT pg_advisory_lock(187200107)"); err != nil {
		return err
	}
	defer func() {
		c, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		if _, e := guard.Exec(c, "SELECT pg_advisory_unlock(187200107)"); e != nil {
			_ = guard.Hijack().Close(c)
		}
	}()
	var oldStatus, oldBinding string
	var oldResult []byte
	err = guard.QueryRow(ctx, "SELECT status,binding,result FROM syntelle_conversation.operations WHERE kind=$1 AND request_key=$2", identity, *language).Scan(&oldStatus, &oldBinding, &oldResult)
	if err == nil {
		if oldStatus != "received" || oldBinding != binding {
			return errors.New("existing failed or changed request cannot be retried")
		}
		_, err = os.Stdout.Write(append(oldResult, '\n'))
		return err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return errors.New("verification history unavailable")
	}
	var attempts, characters int
	var halted bool
	if err = guard.QueryRow(ctx, "SELECT count(*),coalesce(sum((request->>'characters')::int),0),coalesce(bool_or(status<>'received' AND (coalesce((diagnostics->>'provider_error')::boolean,false) OR coalesce(result->>'provider_status','')<>'SynthesizingAudioCompleted')),false) FROM syntelle_conversation.operations WHERE kind=$1", identity).Scan(&attempts, &characters, &halted); err != nil {
		return errors.New("verification accounting unavailable")
	}
	if halted || attempts >= 2 || characters+count > 20 {
		return errors.New("verification halted or allowance exhausted")
	}
	operation, err := store.Reserve(ctx, identity, *language, binding, "", "", request, false)
	if err != nil {
		return err
	}
	provider, err := speech.NewAzureProvider(os.Getenv("AZURE_SPEECH_KEY"), "canadacentral")
	var result speech.Result
	if err == nil {
		result, err = provider.Synthesize(ctx, input)
	}
	providerError := err != nil
	if err == nil {
		err = speech.ValidateLaughterResult(result)
	}
	status := "received"
	if err != nil {
		status = "failed"
	}
	result.Region = "canadacentral"
	finish, stop := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer stop()
	if e := store.Finish(finish, operation.ID, status, result, nil, map[string]any{"provider_error": providerError, "application_rejected": !providerError && err != nil, "no_retry": true}, result.Audio); e != nil {
		return errors.New("verification evidence commit failed; no retry")
	}
	if encodeErr := json.NewEncoder(os.Stdout).Encode(result); encodeErr != nil {
		return encodeErr
	}
	if err != nil {
		return errors.New("laughter preparation rejected; inspect the original result without retry or substitution")
	}
	return nil
}
