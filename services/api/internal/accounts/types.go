package accounts

import (
	"errors"
	"syntelle/render-contract"
	"time"
)

// Principal grants product access only. It never grants local-owner privileges.
type Principal struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	AccountID string `json:"account_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	KeyID     string `json:"key_id,omitempty"`
}

type Account struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type Settings = rendercontract.Settings

func Defaults() Settings { return rendercontract.Defaults() }

type Configuration struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	PackageHash string    `json:"package_hash"`
	Settings    Settings  `json:"settings"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

var (
	ErrUnauthorized = errors.New("sign in to continue")
	ErrForbidden    = errors.New("this resource is not available to this principal")
	ErrNotFound     = errors.New("resource not found")
)
