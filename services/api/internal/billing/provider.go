package billing

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var ErrUnavailable = errors.New("sandbox billing is unavailable")
var ErrQuota = errors.New("workspace has no available paid usage")
var ErrConcurrency = errors.New("workspace concurrency limit reached")
var ErrExpired = errors.New("usage lease expired")

type actionContextKey struct{}
type knownActionFailure struct{ error }
type providerRejection struct{ status int }

func (e *providerRejection) Error() string {
	return fmt.Sprintf("sandbox request rejected (%d)", e.status)
}

type period struct {
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
}
type providerData struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	CustomerID     string `json:"customer_id"`
	SubscriptionID string `json:"subscription_id"`
	TransactionID  string `json:"transaction_id"`
	Currency       string `json:"currency_code"`
	Origin         string `json:"origin"`
	CustomData     struct {
		WorkspaceID string `json:"workspace_id"`
		ActionID    string `json:"action_id"`
	} `json:"custom_data"`
	Items []struct {
		Quantity int `json:"quantity"`
		Price    struct {
			ID string `json:"id"`
		} `json:"price"`
	} `json:"items"`
	BillingPeriod   *period `json:"billing_period"`
	CurrentPeriod   *period `json:"current_billing_period"`
	ScheduledChange *struct {
		Action      string    `json:"action"`
		EffectiveAt time.Time `json:"effective_at"`
	} `json:"scheduled_change"`
	Checkout *struct {
		URL string `json:"url"`
	} `json:"checkout"`
	UpdatedAt time.Time `json:"updated_at"`
	Action    string    `json:"action"`
	Type      string    `json:"type"`
	UnitPrice struct {
		Amount   string `json:"amount"`
		Currency string `json:"currency_code"`
	} `json:"unit_price"`
	BillingCycle *struct {
		Interval  string `json:"interval"`
		Frequency int    `json:"frequency"`
	} `json:"billing_cycle"`
	Details struct {
		Totals struct {
			Total string `json:"total"`
		} `json:"totals"`
	} `json:"details"`
}

func (s *Service) provider(ctx context.Context, method, path string, input any) (providerData, error) {
	if !s.cfg.configured() {
		return providerData{}, ErrUnavailable
	}
	var body io.Reader
	var inputBytes []byte
	if input != nil {
		b, err := json.Marshal(input)
		if err != nil {
			return providerData{}, err
		}
		body = bytes.NewReader(b)
		inputBytes = b
	}
	requestID := ""
	if actionID, ok := ctx.Value(actionContextKey{}).(string); ok && s.pool != nil {
		requestID = randomID()
		if inputBytes == nil {
			inputBytes = []byte{}
		}
		if _, err := s.pool.Exec(ctx, `INSERT INTO syntelle_billing.provider_requests(id,action_id,method,path,request) VALUES($1,$2,$3,$4,$5)`, requestID, actionID, method, path, inputBytes); err != nil {
			return providerData{}, err
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, sandboxURL+path, body)
	if err != nil {
		return providerData{}, err
	}
	req.Header.Set("Authorization", "Bearer "+s.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Paddle-Version", "1")
	response, err := s.client.Do(req)
	if err != nil {
		s.recordProvider(ctx, requestID, nil, 0, "transport_incomplete")
		return providerData{}, fmt.Errorf("sandbox request did not complete: %w", err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		s.recordProvider(ctx, requestID, raw, response.StatusCode, "read_incomplete")
		return providerData{}, err
	}
	if err = s.recordProvider(ctx, requestID, raw, response.StatusCode, "received"); err != nil {
		return providerData{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return providerData{}, &providerRejection{status: response.StatusCode}
	}
	var envelope struct {
		Data providerData `json:"data"`
	}
	if err = json.Unmarshal(raw, &envelope); err != nil {
		return providerData{}, errors.New("sandbox returned an invalid response")
	}
	return envelope.Data, nil
}

func (s *Service) recordProvider(ctx context.Context, id string, raw []byte, status int, outcome string) error {
	if id == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	_, err := s.pool.Exec(ctx, `UPDATE syntelle_billing.provider_requests SET response=$2,status=$3,outcome=$4,completed_at=clock_timestamp() WHERE id=$1`, id, raw, status, outcome)
	return err
}
func verifySignature(raw []byte, header, secret string, now time.Time) bool {
	var ts string
	var hashes []string
	for _, part := range strings.Split(header, ";") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch key {
		case "ts":
			if ts != "" {
				return false
			}
			ts = value
		case "h1":
			hashes = append(hashes, value)
		}
	}
	seconds, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || secret == "" {
		return false
	}
	age := now.Sub(time.Unix(seconds, 0))
	if age < -5*time.Second || age > 5*time.Second {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + ":"))
	mac.Write(raw)
	expected := mac.Sum(nil)
	for _, value := range hashes {
		actual, err := hex.DecodeString(value)
		if err == nil && hmac.Equal(expected, actual) {
			return true
		}
	}
	return false
}
