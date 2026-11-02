package campus

import (
	"testing"
	"time"
)

func TestMovementTrackerCalculatesTimeOutsideOnReturn(t *testing.T) {
	tracker := NewMovementTracker()
	left := time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)
	returned := left.Add(2*time.Hour + 17*time.Minute)
	if movement, ok := tracker.Observe("GJ 01-AB.1234", "gate-1", MovementExit, left); !ok || movement.OutsideDurationSeconds != 0 {
		t.Fatalf("exit movement = %#v, ok=%v", movement, ok)
	}
	movement, ok := tracker.Observe("GJ01AB1234", "gate-1", MovementEntry, returned)
	if !ok || movement.OutsideDurationSeconds != int64((2*time.Hour+17*time.Minute).Seconds()) {
		t.Fatalf("return movement = %#v, ok=%v", movement, ok)
	}
	presence := tracker.Presence()
	if len(presence) != 1 || !presence[0].OnCampus || !presence[0].OutsideSince.IsZero() {
		t.Fatalf("presence = %#v, want on campus", presence)
	}
}

func TestMovementTrackerRejectsUnknownDirection(t *testing.T) {
	tracker := NewMovementTracker()
	if _, ok := tracker.Observe("GJ01AB1234", "gate-1", "UNKNOWN", time.Now()); ok {
		t.Fatal("unknown direction should not create movement")
	}
}
