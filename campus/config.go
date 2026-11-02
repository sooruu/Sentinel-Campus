package campus

import (
	"os"
	"strconv"
	"time"
)

type DemoConfig struct {
	Gates           []Gate
	Vehicles        []Vehicle
	Decision        DecisionConfig
	HardwareBackend string
	Secret          string
	AuthToken       string
	Port            string
}

func DefaultDemoConfig() DemoConfig {
	return DemoConfig{
		Gates: []Gate{
			{ID: "gate-1", Name: "Main Gate", Mode: ModeShadow},
			{ID: "gate-2", Name: "Parking Gate", Mode: ModeShadow},
			{ID: "gate-3", Name: "Hostel Gate", Mode: ModeShadow},
		},
		Vehicles: []Vehicle{
			{PlateNorm: "GJ01AB1234", Status: VehicleActive},
			{PlateNorm: "GJ01BL1234", Status: VehicleBlocked},
			{PlateNorm: "GJ01EX1234", Status: VehicleSuspended},
		},
		Decision:        DecisionConfig{MinConfidence: 0.90, MinFrames: 3, Lockout: 10 * time.Second, DedupWindow: 60 * time.Second},
		HardwareBackend: "simulator",
		Secret:          "sentinel-campus-demo-secret",
		AuthToken:       "local-demo-token",
		Port:            "8088",
	}
}

func LoadDemoConfig() DemoConfig {
	c := DefaultDemoConfig()
	if v := os.Getenv("SENTINEL_CAMPUS_PORT"); v != "" {
		c.Port = v
	}
	if v := os.Getenv("SENTINEL_CAMPUS_SECRET"); v != "" {
		c.Secret = v
	}
	if v := os.Getenv("SENTINEL_CAMPUS_AUTH_TOKEN"); v != "" {
		c.AuthToken = v
	}
	if v := os.Getenv("SENTINEL_CAMPUS_HARDWARE"); v == "simulator" || v == "webhook" {
		c.HardwareBackend = v
	}
	if v := os.Getenv("SENTINEL_CAMPUS_AUTO_GATE"); v != "" {
		for i := range c.Gates {
			if c.Gates[i].ID == v {
				c.Gates[i].Mode = ModeAuto
			}
		}
	}
	if v := os.Getenv("SENTINEL_CAMPUS_MIN_CONFIDENCE"); v != "" {
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			c.Decision.MinConfidence = parsed
		}
	}
	return c
}
