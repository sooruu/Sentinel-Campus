package campus

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Command struct {
	GateID    string    `json:"gate_id"`
	Timestamp time.Time `json:"timestamp"`
	Nonce     string    `json:"nonce"`
	Signature string    `json:"signature"`
}

func SignCommand(secret string, command Command) Command {
	command.Signature = signature(secret, command)
	return command
}

func signature(secret string, command Command) string {
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%s\n%d\n%s", command.GateID, command.Timestamp.UnixNano(), command.Nonce)
	return hex.EncodeToString(mac.Sum(nil))
}

type Controller interface {
	Pulse(context.Context, Command, time.Time) error
}

type WebhookController struct {
	client   *http.Client
	baseURL  string
	secret   string
	lockout  time.Duration
	gateID   string
	mu       sync.Mutex
	lockedTo time.Time
	seen     map[string]struct{}
}

func NewWebhookController(baseURL, secret string, lockout time.Duration, gateID string) *WebhookController {
	return &WebhookController{client: &http.Client{Timeout: 2 * time.Second}, baseURL: strings.TrimRight(baseURL, "/"), secret: secret, lockout: lockout, gateID: gateID, seen: map[string]struct{}{}}
}

func (c *WebhookController) Pulse(ctx context.Context, command Command, now time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := validateCommand(c.secret, c.gateID, c.lockout, c.lockedTo, c.seen, command, now); err != nil {
		return err
	}
	body, err := json.Marshal(command)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/pulse", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("hardware controller unavailable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("hardware controller rejected pulse: %s", resp.Status)
	}
	c.seen[command.Nonce] = struct{}{}
	c.lockedTo = now.Add(c.lockout)
	return nil
}

type SimulatorController struct {
	mu       sync.Mutex
	secret   string
	lockout  time.Duration
	gateID   string
	lockedTo time.Time
	seen     map[string]struct{}
	relayOn  bool
}

func NewSimulatorController(secret string, lockout time.Duration) *SimulatorController {
	return NewSimulatorControllerForGate(secret, lockout, "gate-1")
}

func NewSimulatorControllerForGate(secret string, lockout time.Duration, gateID string) *SimulatorController {
	if lockout == 0 {
		lockout = 10 * time.Second
	}
	return &SimulatorController{secret: secret, lockout: lockout, gateID: gateID, seen: map[string]struct{}{}}
}

func (c *SimulatorController) RelayOn() bool { c.mu.Lock(); defer c.mu.Unlock(); return c.relayOn }

func (c *SimulatorController) Pulse(_ context.Context, command Command, now time.Time) (err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	defer func() {
		if err != nil {
			c.relayOn = false
		}
	}()
	if err = validateCommand(c.secret, c.gateID, c.lockout, c.lockedTo, c.seen, command, now); err != nil {
		return err
	}
	c.seen[command.Nonce] = struct{}{}
	c.lockedTo = now.Add(c.lockout)
	c.relayOn = true
	c.relayOn = false
	return nil
}

func validateCommand(secret, gateID string, lockout time.Duration, lockedTo time.Time, seen map[string]struct{}, command Command, now time.Time) error {
	if command.GateID != gateID {
		return errors.New("unknown gate")
	}
	if command.Nonce == "" {
		return errors.New("missing nonce")
	}
	if command.Timestamp.Before(now.Add(-2*time.Second)) || command.Timestamp.After(now.Add(2*time.Second)) {
		return errors.New("stale command")
	}
	if !hmac.Equal([]byte(command.Signature), []byte(signature(secret, command))) {
		return errors.New("invalid signature")
	}
	if _, exists := seen[command.Nonce]; exists {
		return errors.New("replay nonce")
	}
	if now.Before(lockedTo) {
		return errors.New("controller lockout")
	}
	return nil
}
