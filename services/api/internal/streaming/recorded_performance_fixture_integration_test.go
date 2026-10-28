//go:build integration

package streaming

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"syntelle/api/internal/conversation"
)

type recordedPerformance struct {
	Phrase json.RawMessage `json:"phrase"`
	Speech map[string]any  `json:"speech"`
}

func recordedPerformances() ([]recordedPerformance, error) {
	data, err := os.ReadFile(os.Getenv("SYNTELLE_STREAMING_FIXTURE_RECORDED_PERFORMANCE"))
	if err != nil {
		return nil, err
	}
	var rows []recordedPerformance
	if json.Unmarshal(data, &rows) != nil || len(rows) == 0 {
		return nil, errors.New("invalid frozen performance replay")
	}
	return rows, nil
}

type recordedPerformanceDirectorFixture struct{ *streamModelFixture }

func (m *recordedPerformanceDirectorFixture) PhraseDirection() bool { return true }
func (m *recordedPerformanceDirectorFixture) PhraseRequest(string, []conversation.Message) any {
	return map[string]bool{"exact_recorded_performance": true, "provider_calls_zero": true}
}
func (m *recordedPerformanceDirectorFixture) CompleteWithPhrases(ctx context.Context, _ string, _ []conversation.Message, emit func(conversation.PerformancePhrase) error) (conversation.ModelResult, error) {
	rows, err := recordedPerformances()
	if err != nil {
		return conversation.ModelResult{}, err
	}
	m.calls.Add(1)
	texts := []string{}
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return conversation.ModelResult{}, err
		}
		var p conversation.PerformancePhrase
		var meta struct{ Version, Tone string }
		if json.Unmarshal(row.Phrase, &p) != nil || json.Unmarshal(row.Phrase, &meta) != nil {
			return conversation.ModelResult{}, errors.New("invalid recorded phrase")
		}
		p.Version, p.Tone = meta.Version, meta.Tone
		if err := emit(p); err != nil {
			return conversation.ModelResult{}, err
		}
		texts = append(texts, p.Text)
	}
	return conversation.ModelResult{Text: strings.Join(texts, " "), Response: json.RawMessage(`{"exact_recorded_performance":true,"provider_calls":0}`)}, nil
}
