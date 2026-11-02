package campus

import (
	"testing"
	"time"
)

func TestDecisionEngineOpensOnlyExactAuthorisedReadInAuto(t *testing.T) {
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	engine := NewDecisionEngine(
		[]Gate{{ID: "gate-1", Name: "Main Gate", Mode: ModeAuto}},
		[]Vehicle{{PlateNorm: "GJ01AB1234", Status: VehicleActive}},
		DecisionConfig{MinConfidence: 0.90, MinFrames: 3, Lockout: 10 * time.Second},
	)

	d := engine.Decide("gate-1", PlateRead{PlateNorm: "GJ01AB1234", Confidence: 0.95, Frames: 3}, now)
	if d.Kind != DecisionOpen || !d.Pulse {
		t.Fatalf("authorised exact read = %#v, want OPEN with pulse", d)
	}
}

func TestDecisionEngineNeverPulsesUnsafeReadsOrModes(t *testing.T) {
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		gate GateMode
		read PlateRead
		want DecisionKind
	}{
		{"unknown", ModeAuto, PlateRead{PlateNorm: "GJ01ZZ9999", Confidence: 0.99, Frames: 3}, DecisionDenyAlert},
		{"blocked", ModeAuto, PlateRead{PlateNorm: "GJ01BL1234", Confidence: 0.99, Frames: 3}, DecisionDenyAlert},
		{"low confidence", ModeAuto, PlateRead{PlateNorm: "GJ01AB1234", Confidence: 0.89, Frames: 3}, DecisionHoldGuard},
		{"too few frames", ModeAuto, PlateRead{PlateNorm: "GJ01AB1234", Confidence: 0.99, Frames: 2}, DecisionHoldGuard},
		{"shadow", ModeShadow, PlateRead{PlateNorm: "GJ01AB1234", Confidence: 0.99, Frames: 3}, DecisionNoAction},
		{"manual", ModeManual, PlateRead{PlateNorm: "GJ01AB1234", Confidence: 0.99, Frames: 3}, DecisionNoAction},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vehicles := []Vehicle{{PlateNorm: "GJ01AB1234", Status: VehicleActive}, {PlateNorm: "GJ01BL1234", Status: VehicleBlocked}}
			engine := NewDecisionEngine([]Gate{{ID: "gate-1", Name: "Main Gate", Mode: tc.gate}}, vehicles, DecisionConfig{MinConfidence: 0.90, MinFrames: 3})
			d := engine.Decide("gate-1", tc.read, now)
			if d.Kind != tc.want || d.Pulse {
				t.Fatalf("decision = %#v, want kind %s and no pulse", d, tc.want)
			}
		})
	}
}

func TestDecisionEngineLockoutAndDeduplication(t *testing.T) {
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	engine := NewDecisionEngine([]Gate{{ID: "gate-1", Name: "Main Gate", Mode: ModeAuto}}, []Vehicle{{PlateNorm: "GJ01AB1234", Status: VehicleActive}}, DecisionConfig{MinConfidence: 0.90, MinFrames: 3, Lockout: 10 * time.Second, DedupWindow: 60 * time.Second})
	read := PlateRead{PlateNorm: "GJ01AB1234", Confidence: 0.99, Frames: 3}
	first := engine.Decide("gate-1", read, now)
	second := engine.Decide("gate-1", read, now.Add(time.Second))
	if !first.Pulse || second.Pulse || second.Reason != ReasonLockout {
		t.Fatalf("lockout decisions = %#v, %#v", first, second)
	}
	third := engine.Decide("gate-1", read, now.Add(11*time.Second))
	if third.Pulse || third.Reason != ReasonDuplicate {
		t.Fatalf("dedup decision = %#v, want duplicate without pulse", third)
	}
}

func TestAggregatorRequiresThreeReadsAndUsesMajority(t *testing.T) {
	agg := NewAggregator(3, 60*time.Second)
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	reads := []PlateRead{
		{PlateNorm: "GJ01AB1234", Confidence: 0.90, Timestamp: now},
		{PlateNorm: "GJ01AB1234", Confidence: 0.95, Timestamp: now.Add(time.Second)},
		{PlateNorm: "GJ01AX1234", Confidence: 0.99, Timestamp: now.Add(2 * time.Second)},
		{PlateNorm: "GJ01AB1234", Confidence: 0.96, Timestamp: now.Add(3 * time.Second)},
	}
	for i := 0; i < 3; i++ {
		if _, ready := agg.Add(reads[i]); ready {
			t.Fatalf("aggregator became ready after %d reads", i+1)
		}
	}
	final, ready := agg.Add(reads[3])
	if !ready || final.PlateNorm != "GJ01AB1234" || final.Frames != 3 {
		t.Fatalf("aggregate = %#v, ready=%v", final, ready)
	}
}

func TestAggregatorDoesNotClaimThreeFramesWhenWinnerHasOnlyOne(t *testing.T) {
	agg := NewAggregator(3, 60*time.Second)
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	for _, plate := range []string{"GJ01AB1234", "GJ01AB1234", "GJ01AX1234"} {
		_, ready := agg.Add(PlateRead{PlateNorm: plate, Confidence: 0.99, Timestamp: now})
		if ready {
			t.Fatal("aggregator accepted a winner with fewer than three matching reads")
		}
	}
}

func TestMalformedPlateIsHeldButAllowedSeparatorsNormalize(t *testing.T) {
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	engine := NewDecisionEngine([]Gate{{ID: "gate-1", Name: "Main Gate", Mode: ModeAuto}}, []Vehicle{{PlateNorm: "GJ01AB1234", Status: VehicleActive}}, DecisionConfig{MinConfidence: 0.90, MinFrames: 3})
	valid := engine.Decide("gate-1", PlateRead{PlateNorm: "GJ 01-AB.1234", Confidence: 0.99, Frames: 3}, now)
	if !valid.Pulse {
		t.Fatalf("separated valid plate = %#v", valid)
	}
	invalid := engine.Decide("gate-1", PlateRead{PlateNorm: "GJ01@AB1234", Confidence: 0.99, Frames: 3}, now.Add(20*time.Second))
	if invalid.Pulse || invalid.Reason != ReasonInvalid {
		t.Fatalf("malformed plate = %#v", invalid)
	}
}

func TestVehicleRegistryNormalizesAndUpsertsWithoutDuplicates(t *testing.T) {
	engine := NewDecisionEngine(nil, nil, DecisionConfig{})
	first, err := engine.UpsertVehicle(Vehicle{PlateNorm: "GJ 01-AB.1234", Status: VehicleActive, Owner: "VC Office"})
	if err != nil || first.PlateNorm != "GJ01AB1234" {
		t.Fatalf("first upsert = %#v, err=%v", first, err)
	}
	second, err := engine.UpsertVehicle(Vehicle{PlateNorm: "GJ01AB1234", Status: VehicleBlocked, Owner: "Security"})
	if err != nil || second.Status != VehicleBlocked {
		t.Fatalf("second upsert = %#v, err=%v", second, err)
	}
	vehicles := engine.Vehicles()
	if len(vehicles) != 1 || vehicles[0].Status != VehicleBlocked {
		t.Fatalf("vehicles = %#v, want one blocked record", vehicles)
	}
	if !engine.RemoveVehicle("GJ01AB1234") || len(engine.Vehicles()) != 0 {
		t.Fatalf("remove did not clear vehicle registry")
	}
}

func TestCameraRegistryRequiresSafeMetadata(t *testing.T) {
	registry := NewCameraRegistry(nil)
	if _, err := registry.Upsert(Camera{Name: "Gate camera", Campus: "Gandhinagar", StreamURL: "rtsp://camera"}); err == nil {
		t.Fatal("camera without id should fail")
	}
	camera, err := registry.Upsert(Camera{ID: "cam-g1", Name: "Gate 1", Campus: "Gandhinagar", Site: "Main gate", StreamURL: "rtsp://camera", StreamPath: "cam-g1"})
	if err != nil || camera.Protocol != "RTSP" || camera.Status != CameraOnline || camera.Purpose != "SECURITY" {
		t.Fatalf("camera defaults = %#v, err=%v", camera, err)
	}
	if len(registry.Cameras()) != 1 {
		t.Fatalf("cameras = %d, want 1", len(registry.Cameras()))
	}
}

func TestCameraRegistryMarksMissingHeartbeatOffline(t *testing.T) {
	registry := NewCameraRegistry([]Camera{{ID: "cam-1", Name: "Gate", Campus: "Gandhinagar", StreamURL: "rtsp://camera", Status: CameraOnline, LastSeen: time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)}})
	registry.MarkStale(time.Date(2026, 11, 2, 8, 0, 31, 0, time.UTC), 30*time.Second)
	if registry.Cameras()[0].Status != CameraOffline {
		t.Fatalf("camera status = %s, want OFFLINE", registry.Cameras()[0].Status)
	}
}
