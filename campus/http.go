package campus

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type DemoServer struct {
	engine       *DecisionEngine
	secret       string
	authToken    string
	eventsFile   string
	controllers  map[string]Controller
	cameras      *CameraRegistry
	registryFile string
	movements    *MovementTracker
	exportAudits []EvidenceExportAudit
}

const cameraHealthTTL = 30 * time.Second

func NewDemoServer(engine *DecisionEngine, secret string) *DemoServer {
	return NewDemoServerWithToken(engine, secret, "local-demo-token")
}

func NewDemoServerWithToken(engine *DecisionEngine, secret, authToken string) *DemoServer {
	d := &DemoServer{engine: engine, secret: secret, authToken: authToken, eventsFile: os.Getenv("SENTINEL_CAMPUS_EVENTS_FILE"), registryFile: os.Getenv("SENTINEL_CAMPUS_REGISTRY_FILE"), cameras: NewCameraRegistry(nil), movements: NewMovementTracker(), controllers: map[string]Controller{}}
	d.restoreEvents()
	d.restoreRegistry()
	hardware := os.Getenv("SENTINEL_CAMPUS_HARDWARE")
	controllerURL := os.Getenv("SENTINEL_CAMPUS_CONTROLLER_URL")
	for _, gate := range engine.Gates() {
		if hardware == "webhook" && controllerURL != "" {
			d.controllers[gate.ID] = NewWebhookController(controllerURL+"/"+gate.ID, secret, engine.config.Lockout, gate.ID)
		} else {
			d.controllers[gate.ID] = NewSimulatorControllerForGate(secret, engine.config.Lockout, gate.ID)
		}
	}
	return d
}

func (d *DemoServer) Handler() http.Handler {
	mux := http.NewServeMux()
	if dist, ok := campusWebDist(); ok {
		mux.Handle("/", http.FileServer(http.Dir(dist)))
	} else {
		mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(renderGuardConsole(d.authToken)))
		})
	}
	mux.HandleFunc("GET /api/state", d.state)
	mux.HandleFunc("GET /api/recordings", d.recordings)
	mux.HandleFunc("GET /api/evidence/export", d.exportEvidence)
	mux.HandleFunc("GET /api/cameras", d.listCameras)
	mux.HandleFunc("POST /api/cameras", d.createCamera)
	mux.HandleFunc("POST /api/cameras/import", d.importCameras)
	mux.HandleFunc("GET /api/cameras/export", d.exportCameras)
	mux.HandleFunc("POST /api/cameras/", d.cameraAction)
	mux.HandleFunc("GET /api/vehicles", d.listVehicles)
	mux.HandleFunc("POST /api/vehicles", d.upsertVehicle)
	mux.HandleFunc("POST /api/vehicles/import", d.importVehicles)
	mux.HandleFunc("GET /api/vehicles/export", d.exportVehicles)
	mux.HandleFunc("PUT /api/vehicles/", d.updateVehicle)
	mux.HandleFunc("DELETE /api/vehicles/", d.deleteVehicle)
	if recordings, ok := campusRecordingsDir(); ok {
		mux.Handle("/recordings/", http.StripPrefix("/recordings/", http.FileServer(http.Dir(recordings))))
	}
	mux.HandleFunc("POST /api/gates/", d.gateAction)
	return mux
}

func campusWebDist() (string, bool) {
	root := os.Getenv("SENTINEL_CAMPUS_WEB_DIR")
	if root == "" {
		root = filepath.Join("web-campus", "dist")
	}
	if info, err := os.Stat(filepath.Join(root, "index.html")); err == nil && !info.IsDir() {
		return root, true
	}
	return "", false
}

func (d *DemoServer) ServeHTTP(w http.ResponseWriter, r *http.Request) { d.Handler().ServeHTTP(w, r) }

func (d *DemoServer) state(w http.ResponseWriter, _ *http.Request) {
	d.cameras.MarkStale(time.Now().UTC(), cameraHealthTTL)
	hardware := os.Getenv("SENTINEL_CAMPUS_HARDWARE")
	if hardware == "" {
		hardware = "simulator"
	}
	writeJSON(w, http.StatusOK, map[string]any{"gates": d.engine.Gates(), "vehicles": d.engine.Vehicles(), "cameras": d.cameraViews(), "movements": d.movements.Movements(), "presence": d.movements.Presence(), "alerts": d.alerts(), "exports": d.exportAudits, "events": d.engine.Events(), "hardware_backend": hardware})
}

type cameraView struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	Campus           string       `json:"campus"`
	Site             string       `json:"site"`
	Protocol         string       `json:"protocol"`
	Purpose          string       `json:"purpose"`
	Status           CameraStatus `json:"status"`
	LiveURL          string       `json:"live_url,omitempty"`
	StreamConfigured bool         `json:"stream_configured"`
	LastSeen         time.Time    `json:"last_seen,omitempty"`
}

func (d *DemoServer) cameraViews() []cameraView {
	d.cameras.MarkStale(time.Now().UTC(), cameraHealthTTL)
	cameras := d.cameras.Cameras()
	out := make([]cameraView, 0, len(cameras))
	for _, camera := range cameras {
		out = append(out, d.cameraView(camera))
	}
	return out
}

func (d *DemoServer) listCameras(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, d.cameraViews())
}

func (d *DemoServer) createCamera(w http.ResponseWriter, r *http.Request) {
	if !d.authorized(r) {
		http.Error(w, "missing or invalid Sentinel Campus token", http.StatusUnauthorized)
		return
	}
	var camera Camera
	if !decodeJSON(w, r, &camera) {
		return
	}
	created, err := d.cameras.Upsert(camera)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	d.persistRegistry()
	writeJSON(w, http.StatusOK, d.cameraView(created))
}

func (d *DemoServer) importCameras(w http.ResponseWriter, r *http.Request) {
	if !d.authorized(r) {
		http.Error(w, "missing or invalid Sentinel Campus token", http.StatusUnauthorized)
		return
	}
	reader := csv.NewReader(r.Body)
	header, err := reader.Read()
	if err != nil || len(header) == 0 {
		http.Error(w, "CSV header is required", http.StatusBadRequest)
		return
	}
	columns := map[string]int{}
	for index, column := range header {
		columns[strings.ToLower(strings.TrimSpace(column))] = index
	}
	for _, required := range []string{"id", "name", "campus", "stream_url"} {
		if _, ok := columns[required]; !ok {
			http.Error(w, "CSV must include "+required, http.StatusBadRequest)
			return
		}
	}
	value := func(row []string, name string) string {
		if index, ok := columns[name]; ok && index < len(row) {
			return strings.TrimSpace(row[index])
		}
		return ""
	}
	cameras := make([]Camera, 0)
	seen := map[string]bool{}
	rowNumber := 1
	for {
		rowNumber++
		row, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			http.Error(w, fmt.Sprintf("CSV row %d: %v", rowNumber, readErr), http.StatusBadRequest)
			return
		}
		camera := Camera{ID: value(row, "id"), Name: value(row, "name"), Campus: value(row, "campus"), Site: value(row, "site"), StreamURL: value(row, "stream_url"), StreamPath: value(row, "stream_path"), Protocol: value(row, "protocol"), Purpose: value(row, "purpose"), Status: CameraStatus(strings.ToUpper(value(row, "status")))}
		if seen[camera.ID] {
			http.Error(w, fmt.Sprintf("CSV row %d: duplicate camera id %q", rowNumber, camera.ID), http.StatusBadRequest)
			return
		}
		created, normalizeErr := d.cameras.Upsert(camera)
		if normalizeErr != nil {
			http.Error(w, fmt.Sprintf("CSV row %d: %v", rowNumber, normalizeErr), http.StatusBadRequest)
			return
		}
		seen[created.ID] = true
		cameras = append(cameras, created)
	}
	d.persistRegistry()
	writeJSON(w, http.StatusOK, map[string]any{"imported": len(cameras), "cameras": d.cameraViews()})
}

func (d *DemoServer) exportCameras(w http.ResponseWriter, r *http.Request) {
	if !d.authorized(r) {
		http.Error(w, "missing or invalid Sentinel Campus token", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="sentinel-campus-cameras.csv"`)
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"id", "name", "campus", "site", "stream_url", "stream_path", "protocol", "purpose", "status"})
	for _, camera := range d.cameras.Cameras() {
		// Stream URLs may contain credentials. Export the inventory shape, but
		// intentionally leave the secret-bearing field blank.
		_ = writer.Write([]string{camera.ID, camera.Name, camera.Campus, camera.Site, "", camera.StreamPath, camera.Protocol, camera.Purpose, string(camera.Status)})
	}
	writer.Flush()
}

func (d *DemoServer) cameraAction(w http.ResponseWriter, r *http.Request) {
	if !d.authorized(r) {
		http.Error(w, "missing or invalid Sentinel Campus token", http.StatusUnauthorized)
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 4 || parts[0] != "api" || parts[1] != "cameras" {
		http.NotFound(w, r)
		return
	}
	cameraID, action := parts[2], parts[3]
	status := CameraOnline
	if action == "status" {
		var in struct {
			Status CameraStatus `json:"status"`
		}
		if !decodeJSON(w, r, &in) {
			return
		}
		status = in.Status
	} else if action != "heartbeat" {
		http.NotFound(w, r)
		return
	}
	camera, ok := d.cameras.SetStatus(cameraID, status, time.Now().UTC())
	if !ok {
		http.Error(w, "camera not found or invalid status", http.StatusBadRequest)
		return
	}
	d.persistRegistry()
	writeJSON(w, http.StatusOK, d.cameraView(camera))
}

type CampusAlert struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Severity  string    `json:"severity"`
	Message   string    `json:"message"`
	CameraID  string    `json:"camera_id,omitempty"`
	GateID    string    `json:"gate_id,omitempty"`
	PlateNorm string    `json:"plate_norm,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

func (d *DemoServer) alerts() []CampusAlert {
	d.cameras.MarkStale(time.Now().UTC(), cameraHealthTTL)
	alerts := make([]CampusAlert, 0)
	for _, camera := range d.cameras.Cameras() {
		if camera.Status != CameraOffline && camera.Status != CameraBlocked {
			continue
		}
		kind := "CAMERA_OFFLINE"
		message := "Camera is offline"
		if camera.Status == CameraBlocked {
			kind, message = "CAMERA_BLOCKED", "Camera may be obstructed or tampered with"
		}
		alerts = append(alerts, CampusAlert{ID: "camera-" + camera.ID, Kind: kind, Severity: "HIGH", Message: message, CameraID: camera.ID, Timestamp: camera.LastSeen})
	}
	for index, event := range d.engine.Events() {
		if event.Reason != ReasonTailgating {
			continue
		}
		alerts = append(alerts, CampusAlert{ID: fmt.Sprintf("tailgating-%d", index), Kind: "TAILGATING", Severity: "HIGH", Message: "Second vehicle detected after an authorised vehicle", GateID: event.GateID, PlateNorm: event.PlateNorm, Timestamp: event.Timestamp})
	}
	return alerts
}

type evidenceBundle struct {
	Version    string    `json:"version"`
	ExportedAt time.Time `json:"exported_at"`
	EventCount int       `json:"event_count"`
	SHA256     string    `json:"sha256"`
	Signature  string    `json:"signature"`
	CSV        string    `json:"csv"`
}

type EvidenceExportAudit struct {
	ID         string    `json:"id"`
	ExportedAt time.Time `json:"exported_at"`
	EventCount int       `json:"event_count"`
	SHA256     string    `json:"sha256"`
}

func (d *DemoServer) exportEvidence(w http.ResponseWriter, r *http.Request) {
	if !d.authorized(r) {
		http.Error(w, "missing or invalid Sentinel Campus token", http.StatusUnauthorized)
		return
	}
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	_ = writer.Write([]string{"timestamp", "camera", "gate", "plate", "direction", "decision", "reason", "message"})
	for _, event := range d.engine.Events() {
		_ = writer.Write([]string{event.Timestamp.Format(time.RFC3339), event.CameraID, event.GateID, event.PlateNorm, string(event.Direction), string(event.Kind), string(event.Reason), event.Message})
	}
	writer.Flush()
	content := buffer.Bytes()
	hash := sha256.Sum256(content)
	exportedAt := time.Now().UTC()
	hashHex := hex.EncodeToString(hash[:])
	mac := hmac.New(sha256.New, []byte(d.secret))
	_, _ = mac.Write([]byte(hashHex + "|" + exportedAt.Format(time.RFC3339Nano)))
	d.exportAudits = append(d.exportAudits, EvidenceExportAudit{ID: "export-" + nonce(), ExportedAt: exportedAt, EventCount: len(d.engine.Events()), SHA256: hashHex})
	d.persistRegistry()
	writeJSON(w, http.StatusOK, evidenceBundle{Version: "sentinel-campus-evidence-v1", ExportedAt: exportedAt, EventCount: len(d.engine.Events()), SHA256: hashHex, Signature: hex.EncodeToString(mac.Sum(nil)), CSV: string(content)})
}

func (d *DemoServer) cameraView(camera Camera) cameraView {
	base := strings.TrimRight(os.Getenv("SENTINEL_CAMPUS_LIVE_HLS_BASE"), "/")
	liveURL := ""
	if base != "" && camera.StreamPath != "" {
		liveURL = base + "/" + strings.Trim(camera.StreamPath, "/") + "/index.m3u8"
	}
	return cameraView{ID: camera.ID, Name: camera.Name, Campus: camera.Campus, Site: camera.Site, Protocol: camera.Protocol, Purpose: camera.Purpose, Status: camera.Status, LiveURL: liveURL, StreamConfigured: camera.StreamURL != "", LastSeen: camera.LastSeen}
}

func (d *DemoServer) listVehicles(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, d.engine.Vehicles())
}

func (d *DemoServer) upsertVehicle(w http.ResponseWriter, r *http.Request) {
	if !d.authorized(r) {
		http.Error(w, "missing or invalid Sentinel Campus token", http.StatusUnauthorized)
		return
	}
	var vehicle Vehicle
	if !decodeJSON(w, r, &vehicle) {
		return
	}
	created, err := d.engine.UpsertVehicle(vehicle)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	d.persistRegistry()
	writeJSON(w, http.StatusOK, created)
}

func (d *DemoServer) updateVehicle(w http.ResponseWriter, r *http.Request) {
	if !d.authorized(r) {
		http.Error(w, "missing or invalid Sentinel Campus token", http.StatusUnauthorized)
		return
	}
	plate, err := url.PathUnescape(strings.TrimPrefix(r.URL.Path, "/api/vehicles/"))
	if err != nil || plate == "" {
		http.Error(w, "invalid vehicle plate", http.StatusBadRequest)
		return
	}
	var vehicle Vehicle
	if !decodeJSON(w, r, &vehicle) {
		return
	}
	if vehicle.PlateNorm == "" {
		vehicle.PlateNorm = plate
	}
	normalized, err := normalizeVehicle(vehicle)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if normalized.PlateNorm != normalizePlate(plate) && !d.engine.RemoveVehicle(plate) {
		http.Error(w, "vehicle not found", http.StatusNotFound)
		return
	}
	updated, err := d.engine.UpsertVehicle(normalized)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	d.persistRegistry()
	writeJSON(w, http.StatusOK, updated)
}

func (d *DemoServer) importVehicles(w http.ResponseWriter, r *http.Request) {
	if !d.authorized(r) {
		http.Error(w, "missing or invalid Sentinel Campus token", http.StatusUnauthorized)
		return
	}
	reader := csv.NewReader(r.Body)
	header, err := reader.Read()
	if err != nil || len(header) == 0 {
		http.Error(w, "CSV header is required", http.StatusBadRequest)
		return
	}
	columns := map[string]int{}
	for index, column := range header {
		columns[strings.ToLower(strings.TrimSpace(column))] = index
	}
	if _, ok := columns["plate_norm"]; !ok {
		http.Error(w, "CSV must include plate_norm", http.StatusBadRequest)
		return
	}
	vehicles := make([]Vehicle, 0)
	rowNumber := 1
	for {
		rowNumber++
		row, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			http.Error(w, fmt.Sprintf("CSV row %d: %v", rowNumber, readErr), http.StatusBadRequest)
			return
		}
		value := func(name string) string {
			if index, ok := columns[name]; ok && index < len(row) {
				return strings.TrimSpace(row[index])
			}
			return ""
		}
		status := VehicleStatus(strings.ToUpper(value("status")))
		if status == "" {
			status = VehicleActive
		}
		vehicle, normalizeErr := normalizeVehicle(Vehicle{PlateNorm: value("plate_norm"), Status: status, Label: value("label"), Owner: value("owner"), Department: value("department")})
		if normalizeErr != nil {
			http.Error(w, fmt.Sprintf("CSV row %d: %v", rowNumber, normalizeErr), http.StatusBadRequest)
			return
		}
		vehicles = append(vehicles, vehicle)
	}
	for _, vehicle := range vehicles {
		_, _ = d.engine.UpsertVehicle(vehicle)
	}
	d.persistRegistry()
	writeJSON(w, http.StatusOK, map[string]any{"imported": len(vehicles), "vehicles": vehicles})
}

func (d *DemoServer) exportVehicles(w http.ResponseWriter, r *http.Request) {
	if !d.authorized(r) {
		http.Error(w, "missing or invalid Sentinel Campus token", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="sentinel-campus-vehicles.csv"`)
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"plate_norm", "status", "label", "owner", "department", "valid_from", "valid_to"})
	for _, vehicle := range d.engine.Vehicles() {
		_ = writer.Write([]string{vehicle.PlateNorm, string(vehicle.Status), vehicle.Label, vehicle.Owner, vehicle.Department, vehicle.ValidFrom.Format(time.RFC3339), vehicle.ValidTo.Format(time.RFC3339)})
	}
	writer.Flush()
}

func (d *DemoServer) deleteVehicle(w http.ResponseWriter, r *http.Request) {
	if !d.authorized(r) {
		http.Error(w, "missing or invalid Sentinel Campus token", http.StatusUnauthorized)
		return
	}
	plate, err := url.PathUnescape(strings.TrimPrefix(r.URL.Path, "/api/vehicles/"))
	if err != nil || !d.engine.RemoveVehicle(plate) {
		http.Error(w, "vehicle not found", http.StatusNotFound)
		return
	}
	d.persistRegistry()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (d *DemoServer) recordings(w http.ResponseWriter, r *http.Request) {
	dir, ok := campusRecordingsDir()
	if !ok {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		http.Error(w, "recordings unavailable", http.StatusInternalServerError)
		return
	}
	type recording struct {
		Name       string    `json:"name"`
		URL        string    `json:"url"`
		CameraID   string    `json:"camera_id,omitempty"`
		RecordedAt time.Time `json:"recorded_at"`
		Plates     []string  `json:"plates,omitempty"`
	}
	plateQuery := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("plate")))
	cameraQuery := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("camera")))
	fromQuery, _ := time.Parse(time.RFC3339, r.URL.Query().Get("from"))
	toQuery, _ := time.Parse(time.RFC3339, r.URL.Query().Get("to"))
	out := make([]recording, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !isVideoFile(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		item := recording{Name: entry.Name(), URL: "/recordings/" + entry.Name(), RecordedAt: info.ModTime().UTC()}
		for _, event := range d.engine.Events() {
			if event.RecordingURL == item.URL {
				item.CameraID = event.CameraID
				if event.PlateNorm != "" {
					item.Plates = append(item.Plates, event.PlateNorm)
				}
			}
		}
		if (plateQuery != "" && !containsPlate(item.Plates, plateQuery)) || (cameraQuery != "" && !strings.Contains(strings.ToLower(item.CameraID), cameraQuery)) || (!fromQuery.IsZero() && item.RecordedAt.Before(fromQuery)) || (!toQuery.IsZero() && item.RecordedAt.After(toQuery)) {
			continue
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, out)
}

func containsPlate(plates []string, query string) bool {
	for _, plate := range plates {
		if strings.Contains(plate, query) {
			return true
		}
	}
	return false
}

func campusRecordingsDir() (string, bool) {
	candidates := []string{}
	if configured := os.Getenv("SENTINEL_CAMPUS_RECORDINGS_DIR"); configured != "" {
		candidates = append(candidates, configured)
	}
	candidates = append(candidates, "recordings", filepath.Join("..", "Sentinel", "recrodings"))
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, true
		}
	}
	return "", false
}

func isVideoFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".mp4", ".mov", ".webm", ".mkv":
		return true
	default:
		return false
	}
}

func (d *DemoServer) gateAction(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 4 || parts[0] != "api" || parts[1] != "gates" {
		http.NotFound(w, r)
		return
	}
	gateID, action := parts[2], parts[3]
	if !d.authorized(r) {
		http.Error(w, "missing or invalid Sentinel Campus token", http.StatusUnauthorized)
		return
	}
	switch action {
	case "mode":
		d.changeMode(w, r, gateID)
	case "read":
		d.read(w, r, gateID)
	case "override":
		d.override(w, r, gateID)
	default:
		http.NotFound(w, r)
	}
}

func (d *DemoServer) authorized(r *http.Request) bool {
	provided := r.Header.Get("X-Sentinel-Token")
	return d.authToken != "" && subtle.ConstantTimeCompare([]byte(provided), []byte(d.authToken)) == 1
}

func (d *DemoServer) changeMode(w http.ResponseWriter, r *http.Request, gateID string) {
	var in struct {
		Mode    GateMode `json:"mode"`
		Confirm bool     `json:"confirm"`
	}
	if !decodeJSON(w, r, &in) || !in.Confirm {
		http.Error(w, "mode change requires confirm=true", http.StatusBadRequest)
		return
	}
	if !d.engine.SetMode(gateID, in.Mode) {
		http.Error(w, "invalid gate or mode", http.StatusBadRequest)
		return
	}
	d.addEvent(Event{Decision: Decision{GateID: gateID, Kind: DecisionNoAction, Reason: ReasonMode, Timestamp: time.Now().UTC()}, Message: "mode changed to " + string(in.Mode)})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (d *DemoServer) read(w http.ResponseWriter, r *http.Request, gateID string) {
	var in PlateRead
	if !decodeJSON(w, r, &in) {
		return
	}
	decision := d.engine.Decide(gateID, in, time.Now().UTC())
	_, _ = d.movements.Observe(decision.PlateNorm, gateID, in.Direction, decision.Timestamp)
	d.persistEvents()
	if decision.Pulse && in.VehicleCount > 1 {
		decision.Tailgating = true
		d.addEvent(Event{Decision: Decision{GateID: gateID, PlateNorm: decision.PlateNorm, Direction: decision.Direction, CameraID: decision.CameraID, RecordingURL: decision.RecordingURL, VehicleCount: in.VehicleCount, Kind: DecisionDenyAlert, Reason: ReasonTailgating, Timestamp: decision.Timestamp}, Message: fmt.Sprintf("%d vehicles detected in one authorised gate cycle", in.VehicleCount)})
	}
	d.persistRegistry()
	if decision.Pulse {
		controller, ok := d.controllers[gateID]
		if !ok {
			http.Error(w, "controller unavailable", http.StatusServiceUnavailable)
			return
		}
		command := SignCommand(d.secret, Command{GateID: gateID, Timestamp: decision.Timestamp, Nonce: nonce()})
		if err := controller.Pulse(r.Context(), command, decision.Timestamp); err != nil {
			d.addEvent(Event{Decision: Decision{GateID: gateID, PlateNorm: decision.PlateNorm, Kind: DecisionNoAction, Reason: ReasonFault, Timestamp: decision.Timestamp}, Message: err.Error()})
			decision.Kind, decision.Reason, decision.Pulse = DecisionNoAction, ReasonFault, false
		}
	}
	writeJSON(w, http.StatusOK, decision)
}

func (d *DemoServer) override(w http.ResponseWriter, r *http.Request, gateID string) {
	var in struct {
		Action  string `json:"action"`
		Reason  string `json:"reason"`
		Confirm bool   `json:"confirm"`
	}
	if !decodeJSON(w, r, &in) || !in.Confirm {
		http.Error(w, "override requires confirm=true", http.StatusBadRequest)
		return
	}
	if in.Action != "OPEN" && in.Action != "HOLD" {
		http.Error(w, "action must be OPEN or HOLD", http.StatusBadRequest)
		return
	}
	now := time.Now().UTC()
	event := Event{Decision: Decision{GateID: gateID, Kind: DecisionNoAction, Reason: ReasonMode, Timestamp: now}, Message: "guard override: " + in.Action + " — " + in.Reason}
	if in.Action == "OPEN" {
		controller, ok := d.controllers[gateID]
		if !ok {
			http.Error(w, "controller unavailable", http.StatusServiceUnavailable)
			return
		}
		command := SignCommand(d.secret, Command{GateID: gateID, Timestamp: now, Nonce: nonce()})
		if err := controller.Pulse(context.Background(), command, now); err != nil {
			event.Message += ": " + err.Error()
		} else {
			event.Pulse = true
			event.Kind = DecisionOpen
		}
	}
	d.addEvent(event)
	writeJSON(w, http.StatusOK, event)
}

func (d *DemoServer) addEvent(event Event) {
	d.engine.AddEvent(event)
	d.persistEvents()
}

func (d *DemoServer) persistEvents() {
	if d.eventsFile == "" {
		return
	}
	data, err := json.MarshalIndent(d.engine.Events(), "", "  ")
	if err != nil {
		return
	}
	tmp := d.eventsFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err == nil {
		_ = os.Rename(tmp, d.eventsFile)
	}
}

type registrySnapshot struct {
	Vehicles     []Vehicle             `json:"vehicles"`
	Cameras      []Camera              `json:"cameras"`
	Movements    []Movement            `json:"movements"`
	Presence     []VehiclePresence     `json:"presence"`
	ExportAudits []EvidenceExportAudit `json:"export_audits"`
}

func (d *DemoServer) persistRegistry() {
	if d.registryFile == "" {
		return
	}
	data, err := json.MarshalIndent(registrySnapshot{Vehicles: d.engine.Vehicles(), Cameras: d.cameras.Cameras(), Movements: d.movements.Movements(), Presence: d.movements.Presence(), ExportAudits: d.exportAudits}, "", "  ")
	if err != nil {
		return
	}
	tmp := d.registryFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err == nil {
		_ = os.Rename(tmp, d.registryFile)
	}
}

func (d *DemoServer) restoreRegistry() {
	if d.registryFile == "" {
		return
	}
	data, err := os.ReadFile(d.registryFile)
	if err != nil {
		return
	}
	var snapshot registrySnapshot
	if json.Unmarshal(data, &snapshot) != nil {
		return
	}
	for _, vehicle := range snapshot.Vehicles {
		_, _ = d.engine.UpsertVehicle(vehicle)
	}
	d.cameras.Restore(snapshot.Cameras)
	d.movements.Restore(snapshot.Movements, snapshot.Presence)
	d.exportAudits = append([]EvidenceExportAudit(nil), snapshot.ExportAudits...)
}

func (d *DemoServer) restoreEvents() {
	if d.eventsFile == "" {
		return
	}
	data, err := os.ReadFile(d.eventsFile)
	if err != nil {
		return
	}
	var events []Event
	if json.Unmarshal(data, &events) == nil {
		d.engine.RestoreEvents(events)
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func nonce() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
