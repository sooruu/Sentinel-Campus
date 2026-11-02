package campus

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSimulatorStartsOffAndAcceptsOneValidPulse(t *testing.T) {
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	c := NewSimulatorController("secret", 10*time.Second)
	if c.RelayOn() {
		t.Fatal("relay must be off at boot")
	}
	cmd := SignCommand("secret", Command{GateID: "gate-1", Timestamp: now, Nonce: "n1"})
	if err := c.Pulse(context.Background(), cmd, now); err != nil {
		t.Fatal(err)
	}
	if c.RelayOn() {
		t.Fatal("relay must return off after simulated pulse")
	}
}

func TestWebhookControllerPostsSignedPulse(t *testing.T) {
	var received Command
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	c := NewWebhookController(server.URL, "secret", 10*time.Second, "gate-1")
	command := SignCommand("secret", Command{GateID: "gate-1", Timestamp: now, Nonce: "webhook-1"})
	if err := c.Pulse(context.Background(), command, now); err != nil {
		t.Fatal(err)
	}
	if received.Nonce != command.Nonce || received.Signature == "" {
		t.Fatalf("received command = %#v", received)
	}
}

func TestControllerRejectsStaleReplayAndBadSignature(t *testing.T) {
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	c := NewSimulatorController("secret", 10*time.Second)
	stale := SignCommand("secret", Command{GateID: "gate-1", Timestamp: now.Add(-3 * time.Second), Nonce: "stale"})
	if err := c.Pulse(context.Background(), stale, now); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("stale command error = %v", err)
	}
	valid := SignCommand("secret", Command{GateID: "gate-1", Timestamp: now, Nonce: "replay"})
	if err := c.Pulse(context.Background(), valid, now); err != nil {
		t.Fatal(err)
	}
	if err := c.Pulse(context.Background(), valid, now); err == nil || !strings.Contains(err.Error(), "replay") {
		t.Fatalf("replay error = %v", err)
	}
	bad := valid
	bad.Signature = "bad"
	if err := c.Pulse(context.Background(), bad, now); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("bad signature error = %v", err)
	}
}

func TestControllerEnforcesLockoutAndTurnsOffOnError(t *testing.T) {
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	c := NewSimulatorController("secret", 10*time.Second)
	one := SignCommand("secret", Command{GateID: "gate-1", Timestamp: now, Nonce: "one"})
	if err := c.Pulse(context.Background(), one, now); err != nil {
		t.Fatal(err)
	}
	two := SignCommand("secret", Command{GateID: "gate-1", Timestamp: now.Add(time.Second), Nonce: "two"})
	if err := c.Pulse(context.Background(), two, now.Add(time.Second)); err == nil || !strings.Contains(err.Error(), "lockout") {
		t.Fatalf("lockout error = %v", err)
	}
	if c.RelayOn() {
		t.Fatal("relay must be off after rejected command")
	}
	unknown := SignCommand("secret", Command{GateID: "gate-2", Timestamp: now.Add(11 * time.Second), Nonce: "three"})
	if err := c.Pulse(context.Background(), unknown, now.Add(11*time.Second)); err == nil {
		t.Fatal("unknown gate should fail")
	}
	if c.RelayOn() {
		t.Fatal("relay must be off after controller error")
	}
}
