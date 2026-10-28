package streaming

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"syntelle/api/internal/accounts"
	"syntelle/api/internal/conversation"
	"syntelle/render-contract"
)

type ResolveFunc func(http.ResponseWriter, *http.Request, bool) (accounts.Principal, error)

type Config struct {
	LaunchClient      *http.Client
	WorkerToken       string
	MaximumSessions   int
	PackageHash       string
	ValidatePrincipal func(context.Context, accounts.Principal) error
	Admission         AdmissionConfig
	DisableTimeLimits bool
}

// Envelope never contains provider credentials. Conversation audio exists only
// on the authenticated worker connection, never on the public player socket.
type Envelope = rendercontract.Envelope

func envelope(kind, id string, value any) Envelope {
	b, _ := json.Marshal(value)
	return Envelope{Type: kind, SessionID: id, Data: b}
}

type Command struct {
	ID          string             `json:"id"`
	Type        string             `json:"type"`
	Amount      *float64           `json:"amount,omitempty"`
	Repetitions *int               `json:"repetitions,omitempty"`
	Text        string             `json:"text,omitempty"`
	Settings    *accounts.Settings `json:"settings,omitempty"`
}

type SessionResponse struct {
	LaughterAvailable bool                       `json:"laughter_available"`
	StartupTimeoutMS  int                        `json:"startup_timeout_ms,omitempty"`
	ICEServers        []rendercontract.ICEServer `json:"ice_servers,omitempty"`
	ID                string                     `json:"id"`
	Ticket            string                     `json:"ticket,omitempty"`
	State             string                     `json:"state"`
	Configured        bool                       `json:"configured"`
	SpeechConfigured  bool                       `json:"speech_configured"`
	Recognition       bool                       `json:"recognition"`
	Settings          accounts.Settings          `json:"settings"`
	RemainingSeconds  int                        `json:"remaining_seconds,omitempty"`
	TimeLimitsEnabled bool                       `json:"time_limits_enabled"`
}

type commandResult struct {
	ID        string `json:"id"`
	TurnID    string `json:"turn_id,omitempty"`
	Duplicate bool   `json:"duplicate"`
}

type streamSession struct {
	coordinated             bool
	requestedPolicyRevision uint64
	appliedSettings         *rendercontract.Settings
	deliveryWaiters         map[string]chan rendercontract.DeliveryNotice
	reservedDelivery        *rendercontract.DeliveryReservation
	emotionInstance         string
	retiredEmotionInstances []string

	emotionPolicyRevision       uint64
	lastEmotionDecisionSequence uint64

	lastEmotionTurn          string
	lastEmotionPhrase        int
	lastEmotionGeneration    uint64
	id, cid                  string
	workerID                 string
	principal                accounts.Principal
	settings                 accounts.Settings
	state                    string
	created, lastSeen        time.Time
	lastMessage              time.Time
	deadline                 time.Time
	ticketHash               [32]byte
	ticketExpires            time.Time
	ticketUsed               bool
	currentTurn, pendingTurn string
	previewTurn              string
	ignoredTurns             []string
	channel                  *conversation.Channel
	player                   *peer
	playerAttached           bool
	signals                  []Envelope
	cancel                   context.CancelFunc
	commandMu                chan struct{}
}
