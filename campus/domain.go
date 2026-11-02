package campus

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type GateMode string

const (
	ModeShadow GateMode = "SHADOW"
	ModeAuto   GateMode = "AUTO"
	ModeManual GateMode = "MANUAL"
)

type VehicleStatus string

const (
	VehicleActive    VehicleStatus = "ACTIVE"
	VehicleBlocked   VehicleStatus = "BLOCKED"
	VehicleSuspended VehicleStatus = "SUSPENDED"
)

type Gate struct {
	ID   string   `json:"id"`
	Name string   `json:"name"`
	Mode GateMode `json:"mode"`
}

type Vehicle struct {
	PlateNorm  string        `json:"plate_norm"`
	Status     VehicleStatus `json:"status"`
	Label      string        `json:"label,omitempty"`
	Owner      string        `json:"owner,omitempty"`
	Department string        `json:"department,omitempty"`
	ValidFrom  time.Time     `json:"valid_from,omitempty"`
	ValidTo    time.Time     `json:"valid_to,omitempty"`
}

type CameraStatus string

const (
	CameraOnline      CameraStatus = "ONLINE"
	CameraDegraded    CameraStatus = "DEGRADED"
	CameraOffline     CameraStatus = "OFFLINE"
	CameraMaintenance CameraStatus = "MAINTENANCE"
	CameraBlocked     CameraStatus = "BLOCKED"
)

type Camera struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	Campus     string       `json:"campus"`
	Site       string       `json:"site"`
	StreamURL  string       `json:"stream_url"`
	StreamPath string       `json:"stream_path,omitempty"`
	Protocol   string       `json:"protocol"`
	Purpose    string       `json:"purpose"`
	Status     CameraStatus `json:"status"`
	LastSeen   time.Time    `json:"last_seen,omitempty"`
}

type PlateRead struct {
	PlateRaw     string            `json:"plate_raw,omitempty"`
	PlateNorm    string            `json:"plate_norm"`
	Direction    MovementDirection `json:"direction,omitempty"`
	CameraID     string            `json:"camera_id,omitempty"`
	RecordingURL string            `json:"recording_url,omitempty"`
	VehicleCount int               `json:"vehicle_count,omitempty"`
	Confidence   float64           `json:"confidence"`
	Frames       int               `json:"frames"`
	Timestamp    time.Time         `json:"timestamp"`
}

type DecisionKind string

const (
	DecisionOpen      DecisionKind = "OPEN"
	DecisionHoldGuard DecisionKind = "HOLD_GUARD"
	DecisionDenyAlert DecisionKind = "DENY_ALERT"
	DecisionNoAction  DecisionKind = "NO_ACTION"
)

type DecisionReason string

const (
	ReasonAuthorised    DecisionReason = "AUTHORISED"
	ReasonUnknown       DecisionReason = "UNKNOWN"
	ReasonBlocked       DecisionReason = "BLOCKED"
	ReasonExpired       DecisionReason = "EXPIRED"
	ReasonLowConfidence DecisionReason = "LOW_CONFIDENCE"
	ReasonMode          DecisionReason = "MODE_NOT_AUTO"
	ReasonLockout       DecisionReason = "LOCKOUT"
	ReasonDuplicate     DecisionReason = "DUPLICATE"
	ReasonFault         DecisionReason = "FAULT"
	ReasonInvalid       DecisionReason = "INVALID_PLATE"
	ReasonTailgating    DecisionReason = "TAILGATING"
)

type Decision struct {
	GateID       string            `json:"gate_id"`
	PlateNorm    string            `json:"plate_norm"`
	Direction    MovementDirection `json:"direction,omitempty"`
	CameraID     string            `json:"camera_id,omitempty"`
	RecordingURL string            `json:"recording_url,omitempty"`
	VehicleCount int               `json:"vehicle_count,omitempty"`
	Tailgating   bool              `json:"tailgating,omitempty"`
	Kind         DecisionKind      `json:"decision"`
	Reason       DecisionReason    `json:"reason"`
	Confidence   float64           `json:"confidence"`
	Pulse        bool              `json:"relay_pulsed"`
	Timestamp    time.Time         `json:"timestamp"`
}

type Event struct {
	Decision
	Message string `json:"message,omitempty"`
}

type DecisionConfig struct {
	MinConfidence float64
	MinFrames     int
	Lockout       time.Duration
	DedupWindow   time.Duration
}

type DecisionEngine struct {
	mu       sync.Mutex
	gates    map[string]Gate
	vehicles map[string]Vehicle
	config   DecisionConfig
	lockedTo map[string]time.Time
	lastOpen map[string]time.Time
	events   []Event
}

type CameraRegistry struct {
	mu      sync.Mutex
	cameras map[string]Camera
}

func NewCameraRegistry(cameras []Camera) *CameraRegistry {
	r := &CameraRegistry{cameras: map[string]Camera{}}
	for _, camera := range cameras {
		if camera.ID != "" {
			r.cameras[camera.ID] = camera
		}
	}
	return r
}

func (r *CameraRegistry) Upsert(camera Camera) (Camera, error) {
	camera.ID = strings.TrimSpace(camera.ID)
	camera.Name = strings.TrimSpace(camera.Name)
	camera.Campus = strings.TrimSpace(camera.Campus)
	camera.Site = strings.TrimSpace(camera.Site)
	camera.StreamURL = strings.TrimSpace(camera.StreamURL)
	camera.Protocol = strings.ToUpper(strings.TrimSpace(camera.Protocol))
	camera.Purpose = strings.TrimSpace(camera.Purpose)
	if camera.ID == "" || camera.Name == "" || camera.Campus == "" || camera.StreamURL == "" {
		return Camera{}, fmt.Errorf("camera id, name, campus, and stream_url are required")
	}
	if camera.Protocol == "" {
		camera.Protocol = "RTSP"
	}
	if camera.Purpose == "" {
		camera.Purpose = "SECURITY"
	}
	if camera.Status == "" {
		camera.Status = CameraOnline
	}
	if camera.LastSeen.IsZero() {
		camera.LastSeen = time.Now().UTC()
	}
	if camera.Status != CameraOnline && camera.Status != CameraDegraded && camera.Status != CameraOffline && camera.Status != CameraMaintenance && camera.Status != CameraBlocked {
		return Camera{}, fmt.Errorf("invalid camera status")
	}
	r.mu.Lock()
	r.cameras[camera.ID] = camera
	r.mu.Unlock()
	return camera, nil
}

func (r *CameraRegistry) Cameras() []Camera {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Camera, 0, len(r.cameras))
	for _, camera := range r.cameras {
		out = append(out, camera)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (r *CameraRegistry) Restore(cameras []Camera) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, camera := range cameras {
		if camera.ID != "" {
			r.cameras[camera.ID] = camera
		}
	}
}

func (r *CameraRegistry) SetStatus(id string, status CameraStatus, now time.Time) (Camera, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	camera, ok := r.cameras[id]
	if !ok || (status != CameraOnline && status != CameraDegraded && status != CameraOffline && status != CameraMaintenance && status != CameraBlocked) {
		return Camera{}, false
	}
	camera.Status = status
	if status == CameraOnline {
		camera.LastSeen = now
	}
	r.cameras[id] = camera
	return camera, true
}

func (r *CameraRegistry) MarkStale(now time.Time, ttl time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, camera := range r.cameras {
		if camera.Status == CameraOnline && !camera.LastSeen.IsZero() && now.Sub(camera.LastSeen) > ttl {
			camera.Status = CameraOffline
			r.cameras[id] = camera
		}
	}
}

func NewDecisionEngine(gates []Gate, vehicles []Vehicle, config DecisionConfig) *DecisionEngine {
	if config.MinConfidence == 0 {
		config.MinConfidence = 0.90
	}
	if config.MinFrames == 0 {
		config.MinFrames = 3
	}
	if config.Lockout == 0 {
		config.Lockout = 10 * time.Second
	}
	if config.DedupWindow == 0 {
		config.DedupWindow = 60 * time.Second
	}
	e := &DecisionEngine{gates: map[string]Gate{}, vehicles: map[string]Vehicle{}, config: config, lockedTo: map[string]time.Time{}, lastOpen: map[string]time.Time{}}
	for _, gate := range gates {
		e.gates[gate.ID] = gate
	}
	for _, vehicle := range vehicles {
		e.vehicles[normalizePlate(vehicle.PlateNorm)] = vehicle
	}
	return e
}

func normalizePlate(in string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(strings.TrimSpace(in)) {
		if r == ' ' || r == '-' || r == '.' {
			continue
		}
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		return ""
	}
	norm := b.String()
	if norm == "" || !validPlate(norm) {
		return ""
	}
	return norm
}

func validPlate(norm string) bool {
	if len(norm) >= 2 && norm[0] >= 'A' && norm[0] <= 'Z' && norm[1] >= 'A' && norm[1] <= 'Z' {
		i, digits := 2, 0
		for i < len(norm) && digits < 2 && norm[i] >= '0' && norm[i] <= '9' {
			i++
			digits++
		}
		if digits == 0 {
			return false
		}
		series := 0
		for i < len(norm) && series < 3 && norm[i] >= 'A' && norm[i] <= 'Z' {
			i++
			series++
		}
		return len(norm)-i == 4 && allDigits(norm[i:])
	}
	if len(norm) >= 9 && norm[0] >= '0' && norm[0] <= '9' && norm[1] >= '0' && norm[1] <= '9' && norm[2:4] == "BH" && allDigits(norm[4:8]) {
		return len(norm[8:]) >= 1 && len(norm[8:]) <= 2 && allLetters(norm[8:])
	}
	return false
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
func allLetters(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

func (e *DecisionEngine) SetMode(gateID string, mode GateMode) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	gate, ok := e.gates[gateID]
	if !ok || (mode != ModeShadow && mode != ModeAuto && mode != ModeManual) {
		return false
	}
	gate.Mode = mode
	e.gates[gateID] = gate
	return true
}

func (e *DecisionEngine) Gates() []Gate {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]Gate, 0, len(e.gates))
	for _, gate := range e.gates {
		out = append(out, gate)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (e *DecisionEngine) Events() []Event {
	e.mu.Lock()
	defer e.mu.Unlock()
	start := 0
	if len(e.events) > 20 {
		start = len(e.events) - 20
	}
	out := append([]Event{}, e.events[start:]...)
	return out
}

func (e *DecisionEngine) Vehicles() []Vehicle {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]Vehicle, 0, len(e.vehicles))
	for _, vehicle := range e.vehicles {
		out = append(out, vehicle)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PlateNorm < out[j].PlateNorm })
	return out
}

func (e *DecisionEngine) UpsertVehicle(vehicle Vehicle) (Vehicle, error) {
	var err error
	vehicle, err = normalizeVehicle(vehicle)
	if err != nil {
		return Vehicle{}, err
	}
	e.mu.Lock()
	e.vehicles[vehicle.PlateNorm] = vehicle
	e.mu.Unlock()
	return vehicle, nil
}

func normalizeVehicle(vehicle Vehicle) (Vehicle, error) {
	vehicle.PlateNorm = normalizePlate(vehicle.PlateNorm)
	vehicle.Label = strings.TrimSpace(vehicle.Label)
	vehicle.Owner = strings.TrimSpace(vehicle.Owner)
	vehicle.Department = strings.TrimSpace(vehicle.Department)
	if vehicle.PlateNorm == "" {
		return Vehicle{}, fmt.Errorf("invalid plate")
	}
	if vehicle.Status != VehicleActive && vehicle.Status != VehicleBlocked && vehicle.Status != VehicleSuspended {
		return Vehicle{}, fmt.Errorf("invalid vehicle status")
	}
	return vehicle, nil
}

func (e *DecisionEngine) RemoveVehicle(plate string) bool {
	norm := normalizePlate(plate)
	if norm == "" {
		return false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.vehicles[norm]; !ok {
		return false
	}
	delete(e.vehicles, norm)
	return true
}

func (e *DecisionEngine) AddEvent(event Event) {
	e.mu.Lock()
	e.events = append(e.events, event)
	e.mu.Unlock()
}

func (e *DecisionEngine) RestoreEvents(events []Event) {
	e.mu.Lock()
	e.events = append([]Event(nil), events...)
	e.mu.Unlock()
}

func (e *DecisionEngine) Decide(gateID string, read PlateRead, now time.Time) Decision {
	e.mu.Lock()
	defer e.mu.Unlock()
	d := Decision{GateID: gateID, PlateNorm: normalizePlate(read.PlateNorm), Direction: read.Direction, CameraID: read.CameraID, RecordingURL: read.RecordingURL, VehicleCount: read.VehicleCount, Confidence: read.Confidence, Timestamp: now, Kind: DecisionNoAction}
	gate, ok := e.gates[gateID]
	if !ok {
		d.Reason = ReasonFault
		return e.record(d)
	}
	if gate.Mode != ModeAuto {
		d.Reason = ReasonMode
		return e.record(d)
	}
	if until := e.lockedTo[gateID]; now.Before(until) {
		d.Reason = ReasonLockout
		return e.record(d)
	}
	if d.PlateNorm == "" {
		d.Kind, d.Reason = DecisionHoldGuard, ReasonInvalid
		return e.record(d)
	}
	if read.Confidence < e.config.MinConfidence || read.Frames < e.config.MinFrames {
		d.Kind, d.Reason = DecisionHoldGuard, ReasonLowConfidence
		return e.record(d)
	}
	v, ok := e.vehicles[d.PlateNorm]
	if !ok {
		d.Kind, d.Reason = DecisionDenyAlert, ReasonUnknown
		return e.record(d)
	}
	if v.Status == VehicleBlocked {
		d.Kind, d.Reason = DecisionDenyAlert, ReasonBlocked
		return e.record(d)
	}
	if v.Status != VehicleActive || (!v.ValidFrom.IsZero() && now.Before(v.ValidFrom)) || (!v.ValidTo.IsZero() && !now.Before(v.ValidTo)) {
		d.Kind, d.Reason = DecisionDenyAlert, ReasonExpired
		return e.record(d)
	}
	if opened := e.lastOpen[d.PlateNorm]; !opened.IsZero() && now.Sub(opened) < e.config.DedupWindow {
		d.Reason = ReasonDuplicate
		return e.record(d)
	}
	d.Kind, d.Reason, d.Pulse = DecisionOpen, ReasonAuthorised, true
	e.lockedTo[gateID] = now.Add(e.config.Lockout)
	e.lastOpen[d.PlateNorm] = now
	return e.record(d)
}

func (e *DecisionEngine) record(d Decision) Decision {
	e.events = append(e.events, Event{Decision: d})
	return d
}

type Aggregator struct {
	need   int
	window time.Duration
	reads  []PlateRead
}

func NewAggregator(need int, window time.Duration) *Aggregator {
	if need < 1 {
		need = 1
	}
	return &Aggregator{need: need, window: window}
}

func (a *Aggregator) Add(read PlateRead) (PlateRead, bool) {
	if len(a.reads) > 0 && a.window > 0 && !read.Timestamp.IsZero() && read.Timestamp.Sub(a.reads[0].Timestamp) > a.window {
		a.reads = nil
	}
	a.reads = append(a.reads, read)
	if len(a.reads) < a.need {
		return PlateRead{}, false
	}
	counts := map[string]int{}
	confidence := map[string]float64{}
	for _, item := range a.reads {
		p := normalizePlate(item.PlateNorm)
		counts[p]++
		confidence[p] += item.Confidence
	}
	winner := ""
	for p, count := range counts {
		if winner == "" || count > counts[winner] || (count == counts[winner] && confidence[p] > confidence[winner]) {
			winner = p
		}
	}
	if counts[winner] < a.need {
		return PlateRead{}, false
	}
	out := PlateRead{PlateNorm: winner, Confidence: confidence[winner] / float64(counts[winner]), Frames: counts[winner], Timestamp: read.Timestamp}
	a.reads = nil
	return out, true
}
