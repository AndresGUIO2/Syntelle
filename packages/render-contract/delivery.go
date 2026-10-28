package rendercontract

import (
	"crypto/sha256"
	"encoding/hex"
)

const DefaultVoice = "en-US-Emma:DragonHDLatestNeural"
const CoordinatedPerformanceVersion = "performance-intent/4"
const DeliveryVersion = "coordinated-delivery/1"

type WordBoundary struct {
	Text       string  `json:"text"`
	TextOffset int     `json:"text_offset_utf16"`
	Length     int     `json:"length_utf16"`
	OffsetMS   float64 `json:"offset_ms"`
	DurationMS float64 `json:"duration_ms"`
}

func SupportedVoice(voice string) bool {
	return voice == DefaultVoice || voice == "en-US-Serena:DragonHDLatestNeural" || voice == "en-US-Ava:DragonHDLatestNeural" || voice == "en-US-Andrew:DragonHDLatestNeural"
}

func NaturalPerformance(version string) bool {
	return version == "performance-intent/3" || version == CoordinatedPerformanceVersion
}

func ValidPurpose(purpose string) bool {
	switch purpose {
	case "explain", "reassure", "ask", "affirm", "disagree", "playful":
		return true
	}
	return false
}

func TextHash(text string) string {
	h := sha256.Sum256([]byte(text))
	return hex.EncodeToString(h[:])
}

// These messages travel only on the authenticated API/worker connection.
// A reservation is not played emotional context or a renderer instruction.
type DeliveryRequest struct {
	RequestID   string         `json:"request_id"`
	TurnID      string         `json:"turn_id"`
	PhraseIndex int            `json:"phrase_index"`
	Revision    uint64         `json:"settings_revision"`
	TextHash    string         `json:"text_hash"`
	Voice       string         `json:"voice"`
	Language    string         `json:"language"`
	Purpose     string         `json:"purpose"`
	Emotion     *EmotionTarget `json:"emotion,omitempty"`
}

func (r DeliveryRequest) Valid() bool {
	_, err := hex.DecodeString(r.TextHash)
	return err == nil && len(r.TextHash) == 64 && r.RequestID != "" && len(r.RequestID) <= 128 && r.TurnID != "" && r.PhraseIndex >= 0 && r.PhraseIndex < 1048576 && r.Revision > 0 && SupportedVoice(r.Voice) && (r.Language == "en-US" || r.Language == "es-CO") && ValidPurpose(r.Purpose) && (r.Emotion == nil || r.Emotion.Valid())
}

type DeliveryReservation struct {
	Expressive bool            `json:"expressive"`
	Version    string          `json:"version"`
	Token      string          `json:"token"`
	Request    DeliveryRequest `json:"request"`
	Instance   string          `json:"instance"`
	Generation uint64          `json:"generation"`
	Effective  EmotionTarget   `json:"effective"`
	Reason     string          `json:"reason"`
}

func (r *DeliveryReservation) Valid() bool {
	return r != nil && r.Version == DeliveryVersion && r.Token != "" && r.Instance != "" && r.Request.Valid() && r.Effective.Valid()
}

type DeliveryNotice struct {
	Revision    uint64               `json:"settings_revision,omitempty"`
	Applied     *Settings            `json:"applied_settings,omitempty"`
	RequestID   string               `json:"request_id"`
	Token       string               `json:"token,omitempty"`
	Status      string               `json:"status"`
	Reason      string               `json:"reason,omitempty"`
	Reservation *DeliveryReservation `json:"reservation,omitempty"`
}

// VoiceDelivery contains validated application values, never freeform markup.
type VoiceDelivery struct {
	Version            string        `json:"version"`
	Purpose            string        `json:"purpose"`
	Emotion            EmotionTarget `json:"emotion"`
	Style              string        `json:"style,omitempty"`
	CapabilityRevision string        `json:"capability_revision"`
}

func (d VoiceDelivery) Valid() bool {
	return d.Version == DeliveryVersion && ValidPurpose(d.Purpose) && d.Emotion.Valid() && d.CapabilityRevision != "" && (d.Style == "" || d.Style == "reassuring" || d.Style == "curious" || d.Style == "amused")
}
