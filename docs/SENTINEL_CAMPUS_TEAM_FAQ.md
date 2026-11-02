# Sentinel Campus team-lead FAQ

This is the short answer sheet for the hardware purchase, pilot review, and
team-lead discussion. The safe answer is always: Sentinel requests an opening;
the certified gate controller owns motion safety.

## Project and automation

### What is Sentinel Campus?

Sentinel Campus is a local gate-access system for NFSU. A camera reads a plate,
the software checks the university vehicle register, and an authorised,
high-confidence match can request the gate to open. The VC cabin sees the
decision, event, and source footage in one console.

### How are we going to automate the gate?

```text
camera → ANPR → vehicle register → decision engine
       → signed command → Raspberry Pi → isolated relay
       → gate controller open input
```

The software does not drive the motor directly. It sends a short, authenticated
open request to the gate controller. The existing gate controller continues to
handle the motor, safety loop, obstacle sensor, end stops, emergency stop, and
manual release.

### What happens when a vehicle arrives?

1. The camera captures the vehicle.
2. ANPR reads the plate across multiple frames.
3. Sentinel checks the exact normalised plate against the register.
4. `AUTHORISED` + clear read + `AUTO` mode requests one pulse.
5. `BLOCKED`, `UNKNOWN`, `SUSPENDED`, invalid, or unclear reads stay closed.
6. The decision is recorded and shown to the guard and VC console.

### Why use multiple frames?

One frame can be blurred or misread. Sentinel waits for agreement across a
configured frame quorum and applies a confidence threshold. If the result is
uncertain, it sends the case to the guard instead of opening automatically.

### What if the network goes down?

The safe default is fail closed to automation and keep manual gate control. The
gate must remain operable by the guard even when the laptop, Pi, or network is
unplugged.

### What if the camera is offline?

No new automatic opening is allowed. The console shows the source as unhealthy
and the guard uses manual control until the camera is restored.

### What if the vehicle register is unavailable?

The system must not assume authorisation. It records a fault and requires guard
review.

### Can the guard override the system?

Yes. The guard can keep the gate manual or use a supervised one-time override.
Every override must be recorded with the operator, time, gate, and reason in the
production audit system.

### How do we prevent a vehicle from following another vehicle through?

The first pilot should record this as a separate safety requirement. It needs a
lane camera plus a loop, beam, radar, or equivalent vehicle-presence sensor. A
plate camera alone cannot reliably prove that only one vehicle passed the
barrier.

### How will we measure time inside the campus?

The system needs an entry event and a corresponding exit event. The first pilot
should designate entry and exit lanes, then pair plate sightings by plate,
camera, and time window. This is not part of the current simulator demo yet.

## Hardware and procurement

### What should we buy today for the demo?

Buy the safe model-gate kit first:

- Raspberry Pi 5, preferably 4GB, with active cooling.
- Official or approved 5V/5A USB-C power supply.
- 64GB or larger high-endurance microSD card.
- Ethernet cable and, if needed, a small Ethernet switch.
- Opto-isolated 3.3V-compatible relay module.
- 5V servo or low-voltage model barrier.
- Separate low-voltage power supply for the servo/model gate.
- LED or buzzer for a visible pulse indication.
- Jumper wires, terminal blocks, fuse, enclosure, and a breadboard or proper
  screw-terminal prototype board.

Raspberry Pi GPIO is 3.3V logic and is not 5V tolerant. Do not connect a gate
input or a 5V signal directly to a GPIO pin. Use isolation and have the hardware
owner review the circuit. [Official Raspberry Pi GPIO documentation](https://www.raspberrypi.com/documentation/computers/raspberry-pi.html)
warns that 5V applied directly to GPIO can permanently damage the board.

### What should we buy only after a site survey?

Do not choose these from a generic shopping list:

- Real gate relay or contactor.
- Gate controller interface cable.
- Obstacle sensor or safety loop.
- Camera lens, mounting height, and night/IR configuration.
- PoE injector or switch wattage.
- Industrial enclosure and power protection.
- Any mains-side wiring or access-control panel.

The site survey must identify the exact gate controller model, open-input
voltage, dry-contact behavior, safety inputs, emergency release, network path,
and power source before purchasing or wiring these items.

### Which Raspberry Pi should we choose?

For the model-gate controller, Raspberry Pi 5 4GB is a practical current choice.
It has the standard 40-pin header, Gigabit Ethernet, and USB-C 5V/5A power
specification. The Pi is the controller adapter, not the preferred location for
heavy multi-camera ANPR inference. [Raspberry Pi's official product brief](https://datasheets.raspberrypi.com/rpi5/raspberry-pi-5-product-brief.pdf)
lists those specifications.

For production ANPR, use a separate edge server or GPU machine running the
Sentinel inference service/Triton. Do not assume one Pi can analyse all three
camera streams at the required accuracy and frame rate.

### What camera should we buy?

For the pilot, choose an IP camera with:

- RTSP output and H.264 support.
- 1080p minimum; higher resolution only after checking compute and bandwidth.
- Fixed position and a plate-sized field of view.
- WDR and IR/night support if the gate operates after dark.
- PoE if the site network supports it.
- A documented stream URL and fixed camera clock behavior.

Do not buy a camera based only on megapixels. The mounting angle, shutter speed,
night exposure, plate size in pixels, and stream compatibility matter more.

### Do we need LiveKit?

Not for the first gate pilot. Sentinel already has MediaMTX with RTSP, HLS, and
WHEP/WebRTC paths. LiveKit can be evaluated later for remote multi-operator
rooms, but it should not delay the gate pilot.

## Safety and responsibility

### Can Sentinel close the barrier?

No. Sentinel requests an open action only. Closing, obstacle detection, force
limiting, end stops, and emergency stop belong to the certified gate controller.

### What is the safe wiring boundary?

```text
Pi GPIO → opto-isolated relay → low-voltage gate OPEN/COM input
Safety loop → gate controller
Obstacle sensor → gate controller
Manual release → gate controller
```

Never connect the Raspberry Pi or relay to mains voltage. A qualified gate
technician must approve and install building-side wiring.

### What are the three operating modes?

- `SHADOW`: read and log; never pulse the gate.
- `AUTO`: an exact authorised match can request one pulse.
- `MANUAL`: guard controls the gate; reads remain visible.

Commissioning starts in `SHADOW`, then enables `AUTO` on one gate only during a
supervised pilot.

### What happens if the relay gets stuck?

The hardware design must use a bounded pulse, a relay that returns to its safe
state, controller lockout, and an independent manual/emergency path. The Pi
must not be the only safety layer.

### What data is stored?

The demo stores a local event ledger and recording references. The production
system needs an approved retention period, role-based access, audit logging,
purpose limitation, and a written university data-protection decision before
deployment.

### What approval is needed before a real pilot?

Approval for the gate survey and single-gate pilot, access to camera and gate
sites, a named vehicle-register owner, a retention decision, and a nominated
hardware/gate safety owner.

## What we can demonstrate today

- Three-gate command console.
- Recorded camera footage.
- Authorised, blocked, and unknown decisions.
- Automatic simulator pulse for an authorised vehicle.
- Event search and CSV export.
- Signed webhook contract for a future Pi controller.

## What must be completed before claiming production automation

- Live camera → ANPR/Triton integration.
- Persistent university vehicle register and role access.
- Camera-health and gate-controller health alerts.
- Physical obstacle/safety sensor commissioning.
- Single-gate supervised pilot and accuracy review.
- Entry/exit pairing and tailgating detection.
- Retention and privacy approval.
