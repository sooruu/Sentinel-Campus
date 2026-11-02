package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/forensicspot/sentinel/campus"
)

func main() {
	cfg := campus.LoadDemoConfig()
	engine := campus.NewDecisionEngine(cfg.Gates, cfg.Vehicles, cfg.Decision)
	server := &http.Server{Addr: "127.0.0.1:" + cfg.Port, Handler: campus.NewDemoServerWithToken(engine, cfg.Secret, cfg.AuthToken).Handler()}
	slog.Info("Sentinel Campus demo ready", "url", "http://localhost:"+cfg.Port, "hardware", cfg.HardwareBackend, "auto_gate", os.Getenv("SENTINEL_CAMPUS_AUTO_GATE"), "auth_token", cfg.AuthToken)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("demo stopped", "error", err)
		os.Exit(1)
	}
}
