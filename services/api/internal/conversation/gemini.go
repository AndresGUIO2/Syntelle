package conversation

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const GeminiChatModel = "gemini-3.8-flash"
const geminiEndpoint = "https://generativelanguage.googleapis.com/v1beta/interactions"

type Gemini struct {
	key, model, endpoint string
	client               *http.Client
	directed             bool
}

func NewGemini(key, model string) (*Gemini, error) {
	if strings.TrimSpace(key) == "" {
		return nil, errors.New("Configure GEMINI_API_KEY on the local server.")
	}
	if model == "" {
		model = GeminiChatModel
	}
	if model != GeminiChatModel {
		return nil, errors.New("This conversation adapter requires gemini-3.8-flash; no model fallback is allowed.")
	}
	return &Gemini{key: key, model: model, endpoint: geminiEndpoint, client: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (p *Gemini) WithDirection() *Gemini { p.directed = true; return p }

// Deltas are append-only decoded text, never a completed whole-reply snapshot.
// The first stable phrase has its own durable speech-delivery record.
func (p *Gemini) EarlySpeechText() bool      { return false }
func (p *Gemini) Identity() (string, string) { return "gemini", "go-http/gemini-interactions-v1" }
func (p *Gemini) Settings() any {
	return map[string]any{"provider": "gemini", "model": p.model, "endpoint": p.endpoint, "api": "interactions/v1beta", "store": false, "stream": true, "generation_config": p.generationConfig(), "performance_direction": p.directed}
}
func (p *Gemini) generationConfig() any {
	return map[string]any{"thinking_level": "low", "thinking_summaries": "none", "max_output_tokens": 4096}
}
func (p *Gemini) Request(instructions string, messages []Message) any {
	input := make([]map[string]any, 0, len(messages))
	for _, m := range messages {
		kind := "user_input"
		if m.Role == "assistant" {
			kind = "model_output"
		}
		input = append(input, map[string]any{"type": kind, "content": []map[string]string{{"type": "text", "text": m.Content}}})
	}
	prompt := DefaultInstructions + "\n" + instructions
	request := map[string]any{"model": p.model, "input": input, "system_instruction": prompt, "generation_config": p.generationConfig(), "store": false, "stream": true}
	if p.directed {
		prompt = strings.Replace(prompt, "Reply in plain text in", "Write the spoken text in", 1)
		request["system_instruction"] = prompt + "\n" + DirectionInstructions
		request["response_format"] = map[string]any{"type": "text", "mime_type": "application/json", "schema": PerformanceSchema()}
	}
	return request
}
func (p *Gemini) Complete(ctx context.Context, instructions string, messages []Message, delta func(string)) (ModelResult, error) {
	return p.complete(ctx, instructions, messages, delta, nil)
}
func (p *Gemini) CompleteWithOpening(ctx context.Context, instructions string, messages []Message, delta func(string), opening func(string)) (ModelResult, error) {
	return p.complete(ctx, instructions, messages, delta, opening)
}
func (p *Gemini) complete(ctx context.Context, instructions string, messages []Message, delta func(string), opening func(string)) (ModelResult, error) {
	payload, _ := json.Marshal(p.Request(instructions, messages))
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(payload))
	if e != nil {
		return ModelResult{}, e
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", p.key)
	response, e := p.client.Do(req)
	if e != nil {
		return ModelResult{}, errors.New("Gemini connection failed; no retry was made.")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		data, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
		data = []byte(strings.ReplaceAll(string(data), p.key, "[redacted]"))
		if !json.Valid(data) {
			data, _ = json.Marshal(map[string]string{"body": string(data)})
		}
		return ModelResult{Response: data, HTTPStatus: response.StatusCode}, fmt.Errorf("Gemini returned HTTP %d. Check API access and quota; no fallback or retry was made.", response.StatusCode)
	}
	stream := delta
	var structured strings.Builder
	emitted := ""
	opened := false
	if p.directed {
		stream = func(fragment string) {
			structured.WriteString(fragment)
			if !opened && opening != nil {
				if phrase := OpeningPhrase(SpokenText(decodedTextPrefix(structured.String())), false); phrase != "" {
					opened = true
					opening(phrase)
				}
			}
			if text := decodedTextPrefix(structured.String()); len(text) > len(emitted) && strings.HasPrefix(text, emitted) {
				delta(text[len(emitted):])
				emitted = text
			}
		}
	}
	result, err := readGemini(response.Body, p.model, stream)
	result.HTTPStatus = response.StatusCode
	if p.directed {
		raw := result.Text
		result.Text = emitted
		if err == nil {
			intent, decodeErr := decodePerformance(raw)
			if decodeErr != nil {
				return result, decodeErr
			}
			validation := ValidatePerformance(intent, SpokenText(intent.Text))
			result.Text = intent.Text
			result.Performance = &validation
			if !strings.HasPrefix(intent.Text, emitted) {
				return result, errors.New("The completed reply changed its streamed text; playback was stopped.")
			}
			if len(intent.Text) > len(emitted) {
				delta(intent.Text[len(emitted):])
			}
		}
	}
	return result, err
}

// Stream lifecycle completion is mandatory. Only text deltas belonging to a
// model_output step are spoken; thoughts, tool results and signatures are not.
func readGemini(reader io.Reader, model string, delta func(string)) (ModelResult, error) {
	var result ModelResult
	events := []json.RawMessage{}
	completed := false
	steps := map[int]string{}
	var lines []string
	scanner := bufio.NewScanner(io.LimitReader(reader, 2<<20))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	save := func() { result.Response, _ = json.Marshal(map[string]any{"events": events}) }
	dispatch := func() error {
		if len(lines) == 0 {
			return nil
		}
		data := strings.Join(lines, "\n")
		lines = nil
		if data == "[DONE]" {
			return nil
		}
		var event struct {
			Type  string `json:"event_type"`
			Index int    `json:"index"`
			Step  struct {
				Type string `json:"type"`
			} `json:"step"`
			Delta       struct{ Type, Text string } `json:"delta"`
			Interaction struct {
				ID, Model, Status string
				Usage             json.RawMessage
			} `json:"interaction"`
			Metadata struct {
				Usage json.RawMessage `json:"total_usage"`
			} `json:"metadata"`
		}
		if !json.Valid([]byte(data)) || json.Unmarshal([]byte(data), &event) != nil {
			return errors.New("Invalid Gemini stream.")
		}
		if len(events) >= 4096 {
			return errors.New("Gemini stream exceeded the event safety bound.")
		}
		events = append(events, json.RawMessage(data))
		if len(event.Metadata.Usage) > 0 {
			result.Usage = event.Metadata.Usage
		}
		if event.Interaction.ID != "" {
			result.RequestID = event.Interaction.ID
		}
		if event.Interaction.Model != "" && event.Interaction.Model != model {
			return errors.New("Gemini returned a different model identity; the reply was rejected.")
		}
		switch event.Type {
		case "step.start":
			steps[event.Index] = event.Step.Type
		case "step.delta":
			if event.Delta.Type == "text" && steps[event.Index] == "model_output" {
				if len(result.Text)+len(event.Delta.Text) > 65536 {
					return errors.New("Gemini reply exceeded the message safety bound.")
				}
				result.Text += event.Delta.Text
				delta(event.Delta.Text)
			}
		case "interaction.completed":
			if event.Interaction.Status != "completed" {
				return errors.New("Gemini did not complete the reply; no retry was made.")
			}
			completed = true
			if len(event.Interaction.Usage) > 0 {
				result.Usage = event.Interaction.Usage
			}
		case "interaction.failed", "interaction.cancelled", "interaction.incomplete", "error":
			return errors.New("Gemini did not complete the reply; no retry was made.")
		}
		return nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if e := dispatch(); e != nil {
				save()
				return result, e
			}
		} else if strings.HasPrefix(line, "data:") {
			lines = append(lines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if e := dispatch(); e != nil {
		save()
		return result, e
	}
	save()
	if scanner.Err() != nil || !completed || strings.TrimSpace(result.Text) == "" {
		return result, errors.New("Gemini stream ended without a complete reply.")
	}
	return result, nil
}
