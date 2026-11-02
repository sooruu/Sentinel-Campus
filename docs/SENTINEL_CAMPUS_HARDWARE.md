# Sentinel Campus hardware and automation plan

This document defines how Sentinel Campus connects the camera, decision engine,
gate controller, safety sensors, and operator console. It separates what is
safe to demonstrate today from what must be installed and tested before a real
gate is automated.

## 1. System layout

```text
Gate camera(s)
     │ RTSP / HLS
     ▼
Media source / MediaMTX
     │ frames
     ▼
ANPR pipeline (native CPU or Triton GPU)
     │ plate + confidence + frame quorum
     ▼
Campus decision service
     │ exact vehicle register check
     ├── NO ACTION / GUARD REVIEW
     ├── ALERT + event record
     └── signed HMAC pulse
              │ HTTPS webhook / local network
              ▼
Raspberry Pi controller
     │ GPIO → opto-isolated relay
     ▼
Gate controller dry-contact input
     │
     └── safety sensor, end stops, manual release stay in hardware
```

The operator console reads the same decision and event stream. Video remains on
the local network; the control decision and audit metadata are the small data
that can be sent to the console or a central service.

## 2. Current demo versus production target

| Area | Current Campus demo | Production target |
|---|---|---|
| Video | Local MP4 replay | RTSP camera through MediaMTX/WHEP/HLS |
| Plate read | Supervised synthetic read buttons | ANPR worker using native inference or Triton |
| Vehicle list | In-memory demo register | Persistent university vehicle register/watchlist |
| Decision | Go Campus decision engine | Same decision contract behind the live ANPR pipeline |
| Gate output | Simulator by default | Signed webhook to Raspberry Pi or approved controller |
| Obstacle safety | Not controlled by this app | Gate controller safety loop and sensor interlock |
| Events | Local JSON ledger | Persistent database + audit service |
| Operator UI | Campus console | Campus console plus live wall, evidence, health, and role access |

The current demo must remain in `SHADOW` mode unless a person is supervising the
model gate. The Campus application does not replace a certified gate controller,
emergency stop, obstacle sensor, end stop, or manual release.

## 3. Hardware required

- Approved gate controller with a low-voltage dry-contact open input.
- One camera per lane, positioned so the plate is readable before the barrier.
- Raspberry Pi or approved industrial edge controller with wired network.
- Opto-isolated relay module rated for the gate controller input.
- Gate controller obstacle sensor, safety loop, force limiter, end stops, and
  manual release.
- Separate regulated power supplies for the Pi, relay, camera, and gate
  controller as specified by the hardware owner.
- A physical manual-open path that works when Sentinel is unplugged.

## 4. Safe wiring boundary

The Pi must never switch mains voltage. The only software-controlled circuit is
the low-voltage dry contact electrically parallel with the existing open button:

```text
Pi GPIO ──> opto-isolated relay ──> gate OPEN input / COM

Gate safety loop ──> gate controller safety input
Gate obstacle sensor ──> gate controller safety input
Manual release ──> gate controller manual input
```

The relay is a request to open, not permission to ignore safety. The controller
must reject or stop motion when the safety loop is open, an obstacle is present,
an end stop is reached, or manual release is active.

Never connect the Pi, relay, or prototype wiring to mains. A qualified gate
technician owns the building-side wiring and commissioning.

## 5. Hardware service contract

The Campus server supports a signed webhook adapter:

```sh
SENTINEL_CAMPUS_HARDWARE=webhook \
SENTINEL_CAMPUS_CONTROLLER_URL=http://<pi-address>:<port> \
make campus
```

For Gate 1, the server posts the signed command to:

```text
POST http://<pi-address>:<port>/gate-1/pulse
Content-Type: application/json
```

The JSON command contains `gate_id`, `timestamp`, `nonce`, and `signature`.
The Pi service must verify the HMAC, reject stale timestamps and reused nonces,
check physical safety inputs, close the relay for a bounded pulse, and return a
failure when the safety loop, controller, or relay is unavailable.

The Campus server already enforces signature, freshness, replay, and software
lockout checks. The Pi must enforce them again because it is the final actor.

## 6. How automation works

1. A camera supplies a frame or a recording frame.
2. The ANPR pipeline detects a vehicle and reads the plate.
3. The reader waits for the configured frame quorum and confidence threshold.
4. The plate is normalised and checked against the permitted vehicle register.
5. The decision becomes `OPEN`, `HOLD_GUARD`, `DENY_ALERT`, or `NO_ACTION`.
6. Only an exact authorised match in `AUTO` mode can request an opening pulse.
7. Unknown, blocked, suspended, invalid, or low-confidence reads stay closed.
8. The decision is written to the event ledger and shown in the console.
9. A hardware fault becomes a guard-review event; it never silently opens.

## 7. Gate modes

- `SHADOW`: read and log decisions; never send a relay pulse.
- `AUTO`: exact authorised match plus confidence/frame quorum may request a
  pulse.
- `MANUAL`: the guard controls the gate; reads remain visible for audit.

Commissioning must start in `SHADOW`, then move to `AUTO` for one supervised
gate only. Gates 2 and 3 remain manual until the first gate passes review.

## 8. Commissioning checklist

1. Survey camera view, plate size, lighting, network, power, and gate input.
2. Record controller, relay, sensor wiring, and emergency-stop behavior.
3. Test the manual button with Sentinel disconnected.
4. Test obstacle detection and end stops with the relay disconnected.
5. Test the Pi and relay using an LED or dummy load.
6. Start Sentinel in `SHADOW` and compare reads with the guard register.
7. Enable `AUTO` for one gate during a supervised pilot.
8. Test authorised, blocked, unknown, low-confidence, stale, duplicate, and
   hardware-failure cases.
9. Confirm every test produces an event and every rejected case stays closed.
10. Obtain written approval before expanding to the remaining gates.

## 9. Failure behavior

- Network loss: keep the gate in manual or local controller-safe mode.
- Camera offline: no new automatic decisions; alert the operator.
- ANPR unavailable: no automatic opening.
- Vehicle register unavailable: fail closed and require guard review.
- Relay or Pi unavailable: fail closed and preserve manual operation.
- Safety sensor active: the gate controller owns the stop; Sentinel cannot
  override it.
- Sentinel process stopped: physical manual control remains available.
