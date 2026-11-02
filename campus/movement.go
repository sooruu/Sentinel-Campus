package campus

import (
	"sort"
	"strings"
	"sync"
	"time"
)

type MovementDirection string

const (
	MovementEntry MovementDirection = "ENTRY"
	MovementExit  MovementDirection = "EXIT"
)

type Movement struct {
	PlateNorm              string            `json:"plate_norm"`
	GateID                 string            `json:"gate_id"`
	Direction              MovementDirection `json:"direction"`
	Timestamp              time.Time         `json:"timestamp"`
	OutsideSince           time.Time         `json:"outside_since,omitempty"`
	OutsideDurationSeconds int64             `json:"outside_duration_seconds,omitempty"`
}

type VehiclePresence struct {
	PlateNorm     string            `json:"plate_norm"`
	OnCampus      bool              `json:"on_campus"`
	LastGateID    string            `json:"last_gate_id"`
	LastDirection MovementDirection `json:"last_direction"`
	LastSeen      time.Time         `json:"last_seen"`
	OutsideSince  time.Time         `json:"outside_since,omitempty"`
}

type MovementTracker struct {
	mu        sync.Mutex
	presence  map[string]VehiclePresence
	movements []Movement
}

func NewMovementTracker() *MovementTracker {
	return &MovementTracker{presence: map[string]VehiclePresence{}}
}

func (t *MovementTracker) Observe(plate, gateID string, direction MovementDirection, now time.Time) (Movement, bool) {
	plate = normalizePlate(plate)
	direction = MovementDirection(strings.ToUpper(strings.TrimSpace(string(direction))))
	if plate == "" || gateID == "" || (direction != MovementEntry && direction != MovementExit) {
		return Movement{}, false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	current := t.presence[plate]
	movement := Movement{PlateNorm: plate, GateID: gateID, Direction: direction, Timestamp: now}
	if direction == MovementEntry {
		if !current.OutsideSince.IsZero() && !now.Before(current.OutsideSince) {
			movement.OutsideSince = current.OutsideSince
			movement.OutsideDurationSeconds = int64(now.Sub(current.OutsideSince).Seconds())
		}
		current.OnCampus = true
		current.OutsideSince = time.Time{}
	} else {
		current.OnCampus = false
		current.OutsideSince = now
	}
	current.PlateNorm = plate
	current.LastGateID = gateID
	current.LastDirection = direction
	current.LastSeen = now
	t.presence[plate] = current
	t.movements = append(t.movements, movement)
	if len(t.movements) > 200 {
		t.movements = t.movements[len(t.movements)-200:]
	}
	return movement, true
}

func (t *MovementTracker) Movements() []Movement {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]Movement(nil), t.movements...)
}

func (t *MovementTracker) Presence() []VehiclePresence {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]VehiclePresence, 0, len(t.presence))
	for _, item := range t.presence {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PlateNorm < out[j].PlateNorm })
	return out
}

func (t *MovementTracker) Restore(movements []Movement, presence []VehiclePresence) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.movements = append([]Movement(nil), movements...)
	for _, item := range presence {
		if item.PlateNorm != "" {
			t.presence[item.PlateNorm] = item
		}
	}
}
