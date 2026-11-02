# Sentinel Campus Product Checklist

This is the implementation checklist for turning the three-gate demo into a
campus security product that can later onboard every NFSU campus and its
existing security cameras.

## Core control plane

- [x] Overview of gates, events, recordings, health, and hardware posture
- [x] Local decision engine with SHADOW, AUTO, and MANUAL modes
- [x] Guard override with a recorded reason
- [x] Persistent event ledger for the demo
- [x] Search and CSV export of decision events
- [ ] Multi-campus hierarchy: university → campus → site → gate → camera
- [ ] Role-based access: VC, security lead, campus operator, guard, auditor
- [ ] Audit trail for every configuration change and sensitive read

## Camera operations

- [x] Camera onboarding button and form
- [x] Camera registry with campus, site, stream URL, protocol, and purpose
- [x] Camera status: online, degraded, offline, maintenance
- [x] Camera health view and last-seen timestamp
- [ ] ONVIF discovery and credential validation
- [ ] RTSP/ONVIF secret storage outside the registry response
- [ ] Camera calibration: plate zone, direction, lane, and confidence threshold
- [ ] Camera tamper/occlusion alert
- [ ] Camera-to-gate assignment and failover camera
- [ ] Bulk CSV onboarding for a campus camera inventory
- [x] Bulk CSV camera import with validation
- [x] Safe CSV camera export with stream credentials redacted

## Vehicle access control

- [x] Vehicle allowlist/denylist page
- [x] Add a vehicle with plate, status, owner, and department
- [x] Update a vehicle status
- [x] Edit vehicle metadata from the register
- [x] Remove a vehicle from the register
- [x] Plate normalization and validation
- [x] Entry/exit movement ledger with outside-duration calculation on return
- [x] Bulk CSV import with validation
- [x] CSV export of the complete vehicle register
- [ ] Effective dates and expiry warnings in the UI
- [ ] Approval workflow for allowlist changes
- [ ] Bulk CSV import with duplicate and invalid-row report
- [ ] Temporary visitor passes with one-time or time-window access
- [ ] Per-campus and per-gate access scope
- [ ] Denylist reason, evidence reference, and review status

## Gate automation

- [x] Exact plate + confidence quorum before an automatic pulse
- [x] Unknown, blocked, low-confidence, and stale reads fail closed
- [x] Local simulator controller
- [x] Signed webhook controller adapter
- [x] Manual override path
- [ ] Safety sensor input from the real gate controller
- [x] Follow-through / tailgating detection (demo vehicle-count signal)
- [ ] Gate-open timeout and forced-open alert
- [ ] Interlock and emergency-stop status in the console
- [ ] Offline local queue and replay-safe command IDs
- [ ] Commissioning mode that cannot energise a live gate accidentally

## Video, evidence, and alerts

- [x] Local recording playback
- [x] Recording player controls: play, seek, mute, fullscreen
- [x] MediaMTX-derived live camera preview path
- [x] Event-linked source recording view
- [x] Exportable event ledger
- [x] Search recordings by plate, camera, and day using event-backed metadata
- [x] Evidence package with SHA-256 hash, HMAC manifest, and export audit record
- [x] Camera offline/blocked alerts with authenticated status and heartbeat controls
- [ ] Alert acknowledgement, assignment, escalation, and resolution
- [ ] Retention and deletion policy enforcement
- [ ] Watermarking and access log for exported clips

## NFSU rollout readiness

- [ ] Campus inventory: gates, cameras, VMS, power, network, and controllers
- [ ] Camera compatibility matrix: RTSP, ONVIF, vendor API, codec, and FPS
- [ ] Edge-node sizing per campus
- [ ] Network segmentation and firewall rules
- [ ] Campus-by-campus onboarding runbook
- [ ] Shadow-mode accuracy baseline
- [ ] Pilot acceptance criteria and rollback plan
- [ ] Data protection, retention, and written university approvals
- [ ] Disaster recovery and backup test
- [ ] Operator training and incident playbooks

## Current implementation boundary

The present release implements the first registry slice: camera onboarding,
camera inventory, vehicle allowlist/denylist management, vehicle editing,
validated CSV import/export, persistence, and UI navigation. The remaining
unchecked items are production work, not claims that the demo already controls
every NFSU camera or gate.
