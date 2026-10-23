// Package rendercontract contains only shared wire types and settings.
package rendercontract

import "encoding/json"

type Envelope struct {
	Type      string          `json:"type"`
	SessionID string          `json:"session_id,omitempty"`
	CommandID string          `json:"command_id,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

type Viseme struct {
	ID       int     `json:"id"`
	OffsetMS float64 `json:"offset_ms"`
}

type Signal struct {
	Description *SessionDescription `json:"description,omitempty"`
	Candidate   json.RawMessage     `json:"candidate,omitempty"`
}

type SessionDescription struct {
	Type string `json:"type"`
	SDP  string `json:"sdp"`
}
