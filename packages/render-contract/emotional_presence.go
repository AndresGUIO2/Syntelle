package rendercontract

import "encoding/json"

// Optional prepared capability. Public settings retain normalized intensities.
type EmotionalPresence struct {
	Version      string             `json:"version"`
	SourceSHA    string             `json:"source_sha256"`
	Minimum      float64            `json:"minimum_automatic_intensity"`
	InactivityMS float64            `json:"inactivity_ms"`
	RecoveryMS   float64            `json:"recovery_ms"`
	Gains        map[string]float64 `json:"gains"`
	Validation   struct {
		Valid bool `json:"valid"`
	} `json:"validation"`
}

// MoodCheckpoint is exchanged only on the authenticated worker connection.
// Browser inputs and unactivated delivery reservations cannot populate it.
type MoodCheckpoint struct {
	Version     string          `json:"version"`
	SessionID   string          `json:"session_id"`
	PackageHash string          `json:"package_hash"`
	Instance    string          `json:"instance"`
	Revision    uint64          `json:"revision"`
	Sequence    uint64          `json:"sequence"`
	State       json.RawMessage `json:"state"`
}
