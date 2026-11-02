# Sentinel Campus operator runbook

## One-minute start

From `/Users/sourabh/Projects/Sentinel-Campus`:

```sh
make campus-check
make campus
```

Open <http://localhost:8088>.

For the full preflight → test → build → start flow, use:

```sh
make campus-pipeline
```

## Presentation start

```sh
SENTINEL_CAMPUS_AUTO_GATE=gate-1 make campus
```

Use `Run demo`. It sets the three gates to `AUTO` and creates an authorised,
blocked, and unknown decision. Use `Recordings`, `Events`, and `Evidence` from
the sidebar to show the rest of the workflow.

## Common commands

| Command | Use |
|---|---|
| `make campus` | Build and start the Campus console |
| `make campus-check` | Check Go, Node, frontend, and recordings |
| `make campus-build` | Reinstall and build the Campus frontend |
| `make campus-test` | Run Campus tests and Go vet |
| `make campus-pipeline` | Check, test, build, and start the console |
| `SENTINEL_CAMPUS_PORT=8090 make campus` | Run on another local port |
| `SENTINEL_CAMPUS_RECORDINGS_DIR=/path make campus` | Use another recording folder |

The event ledger is stored in `.sentinel-campus-events.json` and ignored by Git.
Set `SENTINEL_CAMPUS_EVENTS_FILE=/path/events.json` to place it elsewhere.

## Demo pipeline

```text
recording folder → Campus recording API → browser player
→ operator read action → signed Campus decision
→ simulator or webhook controller → event ledger
→ Events / Evidence / CSV export
```

## Production pipeline

```text
camera → RTSP / MediaMTX → frame sampling and tracking
→ native inference or Triton → plate detection and OCR
→ multi-frame confidence quorum → vehicle register/watchlist
→ Campus decision engine → HMAC hardware command
→ Raspberry Pi / gate controller → relay request
→ audit, evidence, alert, and operator UI
```

The original Sentinel services under `internal/`, `cmd/`, and `deploy/` already
contain the production media, ANPR, watchlist, evidence, audit, and health
building blocks. The isolated Campus console uses the first pipeline so it can
run without a database, Triton instance, or physical gate.

## Safe rehearsal

1. Run `make campus-check`.
2. Start with all gates in `SHADOW`.
3. Play a recording from `Recordings`.
4. Click `Run demo` or test one gate manually.
5. Confirm `OPEN` appears only for the authorised plate.
6. Confirm blocked and unknown plates remain closed.
7. Search the event ledger and export it.
8. Open the source from `Evidence`.
9. If real hardware is connected, return to `SHADOW` after rehearsal.

## Production readiness gate

Do not enable real automatic opening until the camera survey, vehicle-list
owner, retention decision, role access, hardware safety test, manual fallback,
one-gate pilot approval, and false-accept/false-reject review are complete.
