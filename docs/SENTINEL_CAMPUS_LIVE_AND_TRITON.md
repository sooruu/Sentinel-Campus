# Sentinel Campus Live Video and Triton Setup

## What the gate controls mean

The controls in each gate card are demo controls for the decision engine:

| Control | What it simulates | Expected result |
|---|---|---|
| Authorised | A clear, repeated read matching an active vehicle | Opens only when the gate is in `AUTO` |
| Blocked | A clear read matching a blocked vehicle | Keeps the gate closed and records an alert |
| Unknown | A clear read not present in the register | Keeps the gate closed and records an alert |
| Open | A supervised guard override | Pulses the controller and records the reason |
| Mode | `SHADOW`, `AUTO`, or `MANUAL` posture | `SHADOW` observes, `AUTO` can pulse, `MANUAL` requires the guard |

The real flow is:

```text
Camera → MediaMTX → ANPR/Triton → vehicle register → safety checks
       → local decision engine → signed gate-controller command → event ledger
```

The UI buttons are not the security decision in production. They are a safe
demo surface for proving the same decision path. The physical gate controller
must still own the obstacle sensor, emergency stop, and manual release.

## Vehicle time outside

Each confirmed plate read carries a movement direction: `ENTRY` or `EXIT`.
The local movement ledger records the gate and timestamp. When the same plate
is next seen entering, the console pairs that entry with the previous exit and
shows the elapsed outside duration on the Vehicles page.

Example:

```text
08:00  GJ01AB1234  EXIT   Main Gate
10:17  GJ01AB1234  ENTRY  Main Gate  → outside for 2h 17m
```

The gate camera must be configured with a direction or lane assignment in the
production camera inventory. The demo exposes this as the “Vehicle entering /
Vehicle leaving” selector on each gate card.

The gate card’s **Tailgate** control simulates a second vehicle count. The first
authorised vehicle can still complete its one pulse, but the second vehicle
creates a high-severity `TAILGATING` alert and never receives a second pulse.

The Evidence page exports a signed JSON bundle containing the event CSV,
SHA-256 digest, HMAC signature, and an append-only local export audit entry.
Production deployment should place the media and manifest in WORM/object-lock
storage and provide an independent verifier.

## Recorded player

The Recordings page provides a custom player for local MP4/MOV/WebM/MKV files:

- play/pause
- seek bar
- mute/unmute
- fullscreen
- source and recording status

The Evidence page links back to the source recording for each event.

## Live camera player

Browsers cannot play an RTSP URL directly. The live path is:

```text
RTSP camera → MediaMTX → HLS browser URL
                         http://127.0.0.1:18888/<stream-path>/index.m3u8
```

When onboarding a camera in the Campus console, set both:

- **Stream URL:** the protected RTSP source used by the edge service
- **MediaMTX stream path:** the path published by MediaMTX, such as `gate-1`

The API stores the RTSP source but never returns it to the browser. It returns
only the derived live preview URL. HLS playback is native on Safari; a later
production browser bundle should add a WebRTC/WHEP player for lower latency
and Chrome/Firefox support.

## Start MediaMTX for a local rehearsal

From the repository root:

```sh
docker compose -f deploy/dev/docker-compose.yml up -d mediamtx
```

To publish one of the existing recordings as a camera-like RTSP feed, install
FFmpeg and run:

```sh
ffmpeg -re -stream_loop -1 \
  -i /Users/sourabh/Projects/Sentinel/recrodings/vid01.mp4 \
  -c copy -f rtsp rtsp://127.0.0.1:8554/gate-1
```

Then onboard a camera with stream path `gate-1`, start the Campus console, and
open `Recordings → Live cameras`.

## Triton Docker path

This repository already contains the GPU Triton overlay. It is intentionally
separate from the default CPU development path because Triton requires an
NVIDIA GPU, NVIDIA drivers, and the NVIDIA Container Toolkit.

On an NVIDIA Linux host:

```sh
./deploy/triton/fetch-all.sh
./deploy/triton/use-gpu.sh
make dev-app-gpu
curl -sf http://127.0.0.1:8000/v2/health/ready && echo "Triton ready"
```

The GPU compose overlay starts `nvcr.io/nvidia/tritonserver:25.02-py3`, mounts
`deploy/triton/models`, loads the configured models explicitly, and exposes the
KServe v2 HTTP endpoint on port `8000`.

On this Apple development machine, the submission/demo path is:

```sh
make dev-app
```

That uses the native CPU inference service. It exercises the same inference
client contract but does not claim that Triton is running locally. The GPU
path must be verified on the target NVIDIA host before making performance or
camera-capacity claims.
