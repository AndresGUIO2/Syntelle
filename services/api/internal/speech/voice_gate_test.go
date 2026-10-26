package speech

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"testing"
)

func TestVoiceGateCountsPeopleAndReleasesTheirLastLease(t *testing.T) {
	var gate voiceGate
	releases := make([]func(), 0, maxActiveVoiceUsers)
	for i := range maxActiveVoiceUsers {
		release, err := gate.acquire(context.Background(), fmt.Sprintf("person-%d", i))
		if err != nil {
			t.Fatal(err)
		}
		releases = append(releases, release)
	}
	secondPhrase, err := gate.acquire(context.Background(), "person-0")
	if err != nil || gate.count() != maxActiveVoiceUsers {
		t.Fatal("a second phrase consumed another person's slot", err)
	}
	if _, err = gate.acquire(context.Background(), "person-50"); !errors.Is(err, errVoiceAtCapacity) {
		t.Fatal("a 51st person was admitted")
	}
	releases[0]()
	if gate.count() != maxActiveVoiceUsers {
		t.Fatal("a person's slot was released while another phrase was active")
	}
	secondPhrase()
	if gate.count() != maxActiveVoiceUsers-1 {
		t.Fatal("completed voice work retained a person's slot")
	}
	replacement, err := gate.acquire(context.Background(), "person-50")
	if err != nil {
		t.Fatal("released slot was not reusable", err)
	}
	replacement()
	for _, release := range releases[1:] {
		release()
	}
	if gate.count() != 0 {
		t.Fatal("voice leases remained after completion")
	}
}

func TestDirectSpeechCapacityRejectsBeforeReservation(t *testing.T) {
	provider := &fakeSpeechProvider{}
	ledger := &fakeSpeechLedger{}
	service := NewService(provider, ledger, "canadacentral")
	releases := make([]func(), 0, maxActiveVoiceUsers)
	for i := range maxActiveVoiceUsers {
		release, err := service.voice.acquire(context.Background(), fmt.Sprintf("person-%d", i))
		if err != nil {
			t.Fatal(err)
		}
		releases = append(releases, release)
	}
	status := httptest.NewRecorder()
	Handler(nil, service).ServeHTTP(status, httptest.NewRequest("GET", "/api/v1/speech/status", nil))
	var capacity struct {
		Active int `json:"active_voice_users"`
		Max    int `json:"max_active_voice_users"`
	}
	if json.Unmarshal(status.Body.Bytes(), &capacity) != nil || capacity.Active != 50 || capacity.Max != 50 {
		t.Fatal("speech status did not report the occupied voice capacity")
	}
	response := postSpeech(Handler(nil, service), validSpeechInput())
	if response.Code != 429 || provider.calls != 0 || ledger.reserveCalls != 0 {
		t.Fatal("voice at capacity reached the provider or speech ledger", response.Code)
	}
	releases[0]()
	response = postSpeech(Handler(nil, service), validSpeechInput())
	if response.Code != 200 || provider.calls != 1 {
		t.Fatal("voice did not recover when a slot was released", response.Code)
	}
	for _, release := range releases[1:] {
		release()
	}
}
