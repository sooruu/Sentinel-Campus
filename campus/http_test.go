package campus

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPStateListsThreeGates(t *testing.T) {
	server := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	var state struct {
		Gates []Gate `json:"gates"`
	}
	if err := json.NewDecoder(res.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	if len(state.Gates) != 3 {
		t.Fatalf("gates = %d, want 3", len(state.Gates))
	}
	if state.Gates[0].ID == "" {
		t.Fatal("first gate has no id")
	}
}

func TestHTTPConsoleUsesExecutiveCommandLayout(t *testing.T) {
	server := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	body := res.Body.String()
	for _, marker := range []string{"command-shell", "VC VIEW", "Gandhinagar / North Campus", "gate-overview"} {
		if !strings.Contains(body, marker) {
			t.Fatalf("console missing premium layout marker %q", marker)
		}
	}
}

func TestHTTPReadPulsesAuthorisedAutoGateAndRejectsUnknown(t *testing.T) {
	server := newTestServer()
	body := `{"plate_norm":"GJ01ZZ9999","confidence":0.99,"frames":3}`
	req := httptest.NewRequest(http.MethodPost, "/api/gates/gate-1/read", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Sentinel-Token", "local-demo-token")
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	var denied Decision
	if err := json.NewDecoder(res.Body).Decode(&denied); err != nil {
		t.Fatal(err)
	}
	if denied.Pulse || denied.Reason != ReasonUnknown {
		t.Fatalf("unknown = %#v", denied)
	}
	body = `{"plate_norm":"GJ01AB1234","confidence":0.99,"frames":3}`
	req = httptest.NewRequest(http.MethodPost, "/api/gates/gate-1/read", strings.NewReader(body))
	req.Header.Set("X-Sentinel-Token", "local-demo-token")
	res = httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("authorised status = %d: %s", res.Code, res.Body.String())
	}
	var decision Decision
	if err := json.NewDecoder(res.Body).Decode(&decision); err != nil {
		t.Fatal(err)
	}
	if !decision.Pulse || decision.Kind != DecisionOpen {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestHTTPModeChangeNeedsConfirmationAndOverrideIsLogged(t *testing.T) {
	server := newTestServer()
	req := httptest.NewRequest(http.MethodPost, "/api/gates/gate-2/mode", strings.NewReader(`{"mode":"AUTO"}`))
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated mode status = %d", res.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/gates/gate-2/mode", strings.NewReader(`{"mode":"AUTO"}`))
	req.Header.Set("X-Sentinel-Token", "local-demo-token")
	res = httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("unconfirmed mode status = %d", res.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/gates/gate-2/mode", strings.NewReader(`{"mode":"AUTO","confirm":true}`))
	req.Header.Set("X-Sentinel-Token", "local-demo-token")
	res = httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("confirmed mode status = %d", res.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/gates/gate-2/override", strings.NewReader(`{"action":"HOLD","reason":"guard check","confirm":true}`))
	req.Header.Set("X-Sentinel-Token", "local-demo-token")
	res = httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("override status = %d", res.Code)
	}
	if len(server.engine.Events()) < 2 {
		t.Fatalf("events = %d, want decision/mode/override records", len(server.engine.Events()))
	}
}

func TestHTTPRegistryRequiresAuthAndDoesNotReturnCameraStreamURL(t *testing.T) {
	t.Setenv("SENTINEL_CAMPUS_LIVE_HLS_BASE", "http://127.0.0.1:18888")
	server := newTestServer()
	request := httptest.NewRequest(http.MethodPost, "/api/cameras", strings.NewReader(`{"id":"cam-g1","name":"Main gate plate camera","campus":"Gandhinagar","site":"Main gate","stream_url":"rtsp://user:secret@camera/stream","stream_path":"cam-g1"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated camera status = %d", response.Code)
	}
	request.Header.Set("X-Sentinel-Token", "local-demo-token")
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "secret") || !strings.Contains(response.Body.String(), "live_url") {
		t.Fatalf("camera response = %d %s", response.Code, response.Body.String())
	}
	request = httptest.NewRequest(http.MethodGet, "/api/cameras", nil)
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "stream_configured") || strings.Contains(response.Body.String(), "rtsp://") {
		t.Fatalf("camera list = %d %s", response.Code, response.Body.String())
	}
}

func TestHTTPCameraRegistrySupportsCSVImportAndSafeExport(t *testing.T) {
	server := newTestServer()
	request := httptest.NewRequest(http.MethodPost, "/api/cameras/import", strings.NewReader("id,name,campus,site,stream_url,stream_path,protocol,purpose,status\ncam-import-1,Parking overview,Gandhinagar,Parking Gate,rtsp://user:secret@camera/stream,parking,RTSP,SECURITY,ONLINE\n"))
	request.Header.Set("X-Sentinel-Token", "local-demo-token")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"imported":1`) {
		t.Fatalf("camera import = %d: %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/api/cameras/export", nil)
	request.Header.Set("X-Sentinel-Token", "local-demo-token")
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(response.Header().Get("Content-Disposition"), "sentinel-campus-cameras.csv") || !strings.Contains(body, "cam-import-1") || !strings.Contains(body, "parking") || strings.Contains(body, "secret") {
		t.Fatalf("camera export = %d: %s", response.Code, body)
	}
}

func TestHTTPVehicleRegistryUpsertsAndDeletes(t *testing.T) {
	server := newTestServer()
	request := httptest.NewRequest(http.MethodPost, "/api/vehicles", strings.NewReader(`{"plate_norm":"GJ 01-XY.4321","status":"BLOCKED","owner":"Security","department":"Campus security"}`))
	request.Header.Set("X-Sentinel-Token", "local-demo-token")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "GJ01XY4321") {
		t.Fatalf("vehicle upsert = %d %s", response.Code, response.Body.String())
	}
	request = httptest.NewRequest(http.MethodDelete, "/api/vehicles/GJ01XY4321", nil)
	request.Header.Set("X-Sentinel-Token", "local-demo-token")
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("vehicle delete = %d %s", response.Code, response.Body.String())
	}
}

func TestHTTPVehicleRegistrySupportsEditCSVImportAndExport(t *testing.T) {
	server := newTestServer()
	request := httptest.NewRequest(http.MethodPost, "/api/vehicles", strings.NewReader(`{"plate_norm":"GJ01ED1111","status":"ACTIVE","label":"Old label"}`))
	request.Header.Set("X-Sentinel-Token", "local-demo-token")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("vehicle create = %d: %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPut, "/api/vehicles/GJ01ED1111", strings.NewReader(`{"plate_norm":"GJ01ED1111","status":"BLOCKED","label":"Updated label","owner":"Registrar"}`))
	request.Header.Set("X-Sentinel-Token", "local-demo-token")
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Updated label") || !strings.Contains(response.Body.String(), "BLOCKED") {
		t.Fatalf("vehicle edit = %d: %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/api/vehicles/import", strings.NewReader("plate_norm,status,label,owner,department\nGJ01IM2222,ACTIVE,Imported vehicle,Library,Operations\n"))
	request.Header.Set("X-Sentinel-Token", "local-demo-token")
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"imported":1`) {
		t.Fatalf("vehicle import = %d: %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/api/vehicles/export", nil)
	request.Header.Set("X-Sentinel-Token", "local-demo-token")
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Header().Get("Content-Disposition"), "sentinel-campus-vehicles.csv") || !strings.Contains(response.Body.String(), "GJ01IM2222") || !strings.Contains(response.Body.String(), "Updated label") {
		t.Fatalf("vehicle export = %d: %s", response.Code, response.Body.String())
	}
}

func TestHTTPReadRecordsVehicleExitAndReturnDuration(t *testing.T) {
	server := newTestServer()
	for _, direction := range []string{"EXIT", "ENTRY"} {
		request := httptest.NewRequest(http.MethodPost, "/api/gates/gate-1/read", strings.NewReader(`{"plate_norm":"GJ01AB1234","confidence":0.99,"frames":3,"direction":"`+direction+`"}`))
		request.Header.Set("X-Sentinel-Token", "local-demo-token")
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s read status = %d: %s", direction, response.Code, response.Body.String())
		}
	}
	request := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	var state struct {
		Movements []Movement        `json:"movements"`
		Presence  []VehiclePresence `json:"presence"`
	}
	if err := json.NewDecoder(response.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	if len(state.Movements) != 2 || state.Movements[0].Direction != MovementExit || state.Movements[1].Direction != MovementEntry {
		t.Fatalf("movements = %#v", state.Movements)
	}
	if len(state.Presence) != 1 || !state.Presence[0].OnCampus {
		t.Fatalf("presence = %#v", state.Presence)
	}
}

func TestHTTPTailgatingCreatesAlertWithoutSecondPulse(t *testing.T) {
	server := newTestServer()
	request := httptest.NewRequest(http.MethodPost, "/api/gates/gate-1/read", strings.NewReader(`{"plate_norm":"GJ01AB1234","confidence":0.99,"frames":3,"vehicle_count":2}`))
	request.Header.Set("X-Sentinel-Token", "local-demo-token")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	var decision Decision
	if err := json.NewDecoder(response.Body).Decode(&decision); err != nil {
		t.Fatal(err)
	}
	if !decision.Pulse || !decision.Tailgating {
		t.Fatalf("decision = %#v, want one pulse plus tailgating alert", decision)
	}
	request = httptest.NewRequest(http.MethodGet, "/api/state", nil)
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	var state struct {
		Alerts []CampusAlert `json:"alerts"`
	}
	if err := json.NewDecoder(response.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	if len(state.Alerts) != 1 || state.Alerts[0].Kind != "TAILGATING" {
		t.Fatalf("alerts = %#v", state.Alerts)
	}
}

func TestHTTPEvidenceExportRequiresAuthAndCarriesIntegrityFields(t *testing.T) {
	server := newTestServer()
	request := httptest.NewRequest(http.MethodGet, "/api/evidence/export", nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated export status = %d", response.Code)
	}
	request.Header.Set("X-Sentinel-Token", "local-demo-token")
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "sha256") || !strings.Contains(response.Body.String(), "signature") {
		t.Fatalf("export = %d %s", response.Code, response.Body.String())
	}
}

func TestHTTPCameraBlockedStatusAppearsAsAlert(t *testing.T) {
	server := newTestServer()
	create := httptest.NewRequest(http.MethodPost, "/api/cameras", strings.NewReader(`{"id":"cam-g1","name":"Gate camera","campus":"Gandhinagar","stream_url":"rtsp://camera"}`))
	create.Header.Set("X-Sentinel-Token", "local-demo-token")
	created := httptest.NewRecorder()
	server.ServeHTTP(created, create)
	status := httptest.NewRequest(http.MethodPost, "/api/cameras/cam-g1/status", strings.NewReader(`{"status":"BLOCKED"}`))
	status.Header.Set("X-Sentinel-Token", "local-demo-token")
	updated := httptest.NewRecorder()
	server.ServeHTTP(updated, status)
	if updated.Code != http.StatusOK {
		t.Fatalf("camera status = %d %s", updated.Code, updated.Body.String())
	}
	state := httptest.NewRecorder()
	server.ServeHTTP(state, httptest.NewRequest(http.MethodGet, "/api/state", nil))
	if !strings.Contains(state.Body.String(), "CAMERA_BLOCKED") {
		t.Fatalf("state missing camera alert: %s", state.Body.String())
	}
}

func newTestServer() *DemoServer {
	engine := NewDecisionEngine([]Gate{{ID: "gate-1", Name: "Main Gate", Mode: ModeAuto}, {ID: "gate-2", Name: "North Gate", Mode: ModeShadow}, {ID: "gate-3", Name: "Service Gate", Mode: ModeShadow}}, []Vehicle{{PlateNorm: "GJ01AB1234", Status: VehicleActive}}, DecisionConfig{MinConfidence: 0.90, MinFrames: 3})
	return NewDemoServer(engine, "demo-secret")
}
