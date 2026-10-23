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
	"net/url"
	"strings"
)

const DefaultInstructions = "Be a clear, friendly assistant. Reply in plain text in the user's selected language. Match the depth and length to the question: explain fully when asked, and keep simple answers brief. Do not invent company knowledge or claim actions you have not performed."

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type ModelResult struct {
	Performance *PerformanceValidation `json:"performance,omitempty"`
	Text        string                 `json:"text"`
	Response    json.RawMessage        `json:"response"`
	Usage       json.RawMessage        `json:"usage"`
	HTTPStatus  int                    `json:"http_status"`
	RequestID   string                 `json:"provider_request_id"`
}

// Director can be implemented by Gemini or another trusted internal controller.
type Director interface {
	Settings() any
	Complete(context.Context, string, []Message, func(string)) (ModelResult, error)
}
type Model = Director
type Responses struct {
	endpoint, deployment, key string
	client                    *http.Client
}

func NewResponses(endpoint, deployment, key string) (*Responses, error) {
	u, e := url.Parse(strings.TrimRight(endpoint, "/"))
	if e != nil || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || !(strings.HasSuffix(u.Hostname(), ".openai.azure.com") || strings.HasSuffix(u.Hostname(), ".services.ai.azure.com")) {
		return nil, errors.New("Use a private Azure OpenAI resource HTTPS endpoint.")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	if !strings.HasSuffix(u.Path, "/openai/v1") {
		u.Path += "/openai/v1"
	}
	u.Path += "/responses"
	if deployment == "" || key == "" {
		return nil, errors.New("Foundry deployment and API key are required.")
	}
	return &Responses{u.String(), deployment, key, &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (p *Responses) Settings() any {
	return map[string]any{"endpoint": p.endpoint, "deployment": p.deployment, "model": "gpt-6-luna", "api": "responses/v1", "stream": true, "store": false, "reasoning": map[string]string{"effort": "none"}, "max_output_tokens": 4096}
}
func (p *Responses) Complete(ctx context.Context, instructions string, messages []Message, delta func(string)) (ModelResult, error) {
	payload := map[string]any{"model": p.deployment, "instructions": DefaultInstructions + "\n" + instructions, "input": messages, "stream": true, "store": false, "reasoning": map[string]string{"effort": "none"}, "max_output_tokens": 4096}
	data, _ := json.Marshal(payload)
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(data))
	if e != nil {
		return ModelResult{}, e
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", p.key)
	resp, e := p.client.Do(req)
	if e != nil {
		return ModelResult{}, errors.New("Foundry connection failed; no retry was made.")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		body = []byte(strings.ReplaceAll(string(body), p.key, "[redacted]"))
		if !json.Valid(body) {
			body, _ = json.Marshal(map[string]string{"body": string(body)})
		}
		return ModelResult{Response: body, HTTPStatus: resp.StatusCode, RequestID: resp.Header.Get("x-request-id")}, fmt.Errorf("Foundry returned HTTP %d. Check the deployment and quota; no fallback or retry was made.", resp.StatusCode)
	}
	result, err := readResponses(resp.Body, delta)
	result.HTTPStatus = resp.StatusCode
	result.RequestID = resp.Header.Get("x-request-id")
	return result, err
}
func readResponses(reader io.Reader, delta func(string)) (ModelResult, error) {
	var result ModelResult
	scanner := bufio.NewScanner(io.LimitReader(reader, 2<<20))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	completed := false
	var lines []string
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
			Type, Delta string
			Response    json.RawMessage
		}
		if json.Unmarshal([]byte(data), &event) != nil {
			return errors.New("Invalid Foundry stream.")
		}
		switch event.Type {
		case "response.output_text.delta":
			if len(result.Text)+len(event.Delta) > 65536 {
				return errors.New("Foundry reply exceeded the per-message safety bound.")
			}
			result.Text += event.Delta
			delta(event.Delta)
		case "response.completed":
			result.Response = event.Response
			var r struct {
				Usage  json.RawMessage
				Status string
			}
			_ = json.Unmarshal(event.Response, &r)
			result.Usage = r.Usage
			completed = r.Status == "completed" || r.Status == ""
		case "response.failed", "response.incomplete", "error":
			result.Response = json.RawMessage(data)
			return errors.New("Foundry did not complete the reply; no retry was made.")
		}
		return nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if e := dispatch(); e != nil {
				return result, e
			}
		} else if strings.HasPrefix(line, "data:") {
			lines = append(lines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if e := dispatch(); e != nil {
		return result, e
	}
	if scanner.Err() != nil || !completed || strings.TrimSpace(result.Text) == "" {
		return result, errors.New("Foundry stream ended without a complete reply.")
	}
	return result, nil
}

// SpokenText normalizes whitespace for speech without discarding any content.
// Original formatting remains in the transcript and immutable model evidence.
func SpokenText(text string) string { return strings.Join(strings.Fields(text), " ") }
func ContextMessages(turns []Turn) []Message {
	if len(turns) > 21 {
		turns = turns[len(turns)-21:]
	}
	out := []Message{}
	for _, t := range turns {
		out = append(out, Message{"user", t.User})
		if t.Reply != "" {
			content := t.Reply
			if t.Status == "interrupted" {
				content += "\n[Playback was interrupted; the user may not have heard all of this reply.]"
			}
			out = append(out, Message{"assistant", content})
		}
	}
	return out
}
