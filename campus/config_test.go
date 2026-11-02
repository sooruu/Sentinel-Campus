package campus

import "testing"

func TestDefaultDemoConfigHasThreeSafeGates(t *testing.T) {
	c := DefaultDemoConfig()
	if len(c.Gates) != 3 {
		t.Fatalf("gates = %d, want 3", len(c.Gates))
	}
	wantNames := []string{"Main Gate", "Parking Gate", "Hostel Gate"}
	for _, gate := range c.Gates {
		if gate.Mode != ModeShadow {
			t.Fatalf("gate %s mode = %s, want SHADOW", gate.ID, gate.Mode)
		}
	}
	for index, want := range wantNames {
		if c.Gates[index].Name != want {
			t.Fatalf("gate %s name = %q, want %q", c.Gates[index].ID, c.Gates[index].Name, want)
		}
	}
	if c.HardwareBackend != "simulator" {
		t.Fatalf("backend = %q, want simulator", c.HardwareBackend)
	}
	if len(c.Vehicles) < 2 {
		t.Fatalf("vehicles = %d, want demo register", len(c.Vehicles))
	}
}
