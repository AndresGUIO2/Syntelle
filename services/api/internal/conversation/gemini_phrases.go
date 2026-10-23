package conversation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func (p *Gemini) PhraseDirection() bool { return p.directed }
func (p *Gemini) PhraseRequest(instructions string, messages []Message) any {
	request := p.Request(instructions, messages).(map[string]any)
	request["system_instruction"] = strings.Replace(DefaultInstructions+"\n"+instructions, "Reply in plain text in", "Write spoken text in", 1) + "\n" + PhraseDirectionInstructions + " Write version and tone before the phrases array."
	request["response_format"] = map[string]any{"type": "text", "mime_type": "application/json", "schema": PhrasePerformanceSchema()}
	return request
}

func (p *Gemini) CompleteWithPhrases(ctx context.Context, instructions string, messages []Message, emit func(PerformancePhrase) error) (ModelResult, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	payload, err := json.Marshal(p.PhraseRequest(instructions, messages))
	if err != nil {
		return ModelResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(payload))
	if err != nil {
		return ModelResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", p.key)
	response, err := p.client.Do(req)
	if err != nil {
		return ModelResult{}, errors.New("Gemini connection failed; no retry was made")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 64<<10))
		if readErr != nil {
			return ModelResult{}, readErr
		}
		safe, _ := json.Marshal(map[string]string{"body": strings.ReplaceAll(string(body), p.key, "[redacted]")})
		return ModelResult{HTTPStatus: response.StatusCode, Response: safe}, fmt.Errorf("Gemini returned HTTP %d; no retry was made", response.StatusCode)
	}
	var buffer strings.Builder
	var delivered []PerformancePhrase
	var streamErr error
	result, err := readGemini(response.Body, p.model, func(fragment string) {
		if streamErr != nil {
			return
		}
		buffer.WriteString(fragment)
		phrases, parseErr := decodedPhrases(buffer.String())
		if parseErr != nil {
			streamErr = parseErr
			cancel()
			return
		}
		if len(phrases) < len(delivered) {
			streamErr = errors.New("phrase stream changed an admitted prefix")
			cancel()
			return
		}
		for _, phrase := range phrases[len(delivered):] {
			if streamErr = emit(phrase); streamErr != nil {
				cancel()
				return
			}
			delivered = append(delivered, phrase)
		}
	})
	result.HTTPStatus = response.StatusCode
	if streamErr != nil {
		return result, streamErr
	}
	if err != nil {
		return result, err
	}
	complete, err := completePhrases(result.Text)
	if err != nil {
		return result, err
	}
	if len(complete) != len(delivered) {
		return result, errors.New("phrase stream changed at completion")
	}
	for i := range complete {
		a, _ := json.Marshal(complete[i])
		b, _ := json.Marshal(delivered[i])
		if !bytes.Equal(a, b) {
			return result, errors.New("completed phrase changed its admitted direction")
		}
	}
	var text []string
	for _, phrase := range complete {
		text = append(text, phrase.Text)
	}
	result.Text = strings.Join(text, " ")
	return result, nil
}
