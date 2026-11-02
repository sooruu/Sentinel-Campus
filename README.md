# Sentinel Campus

Sentinel Campus is a local NFSU campus security console for three gates:
Main Gate, Parking Gate, and Hostel Gate. It provides vehicle plate decisions,
gate controls, camera onboarding, traffic-wall playback, movement tracking,
recordings, evidence export, alerts, and health status.

## Start on Windows

Open PowerShell in this folder:

```powershell
Set-ExecutionPolicy -Scope Process Bypass
.\scripts\start-campus.ps1
```

The launcher installs Go and Node.js LTS through `winget` when required,
installs frontend dependencies, builds the console, and starts it at
`http://localhost:8088`.

For a supervised simulator rehearsal:

```powershell
.\scripts\start-campus.ps1 -AutoGate
```

## Start on macOS or Linux

```sh
./scripts/campus-demo.sh
```

Or:

```sh
make campus
```

All gates start in `SHADOW` mode. The simulator can be selected with
`SENTINEL_CAMPUS_HARDWARE=simulator`; a signed controller webhook can be used
with `SENTINEL_CAMPUS_HARDWARE=webhook` and
`SENTINEL_CAMPUS_CONTROLLER_URL`.

## Included documentation

- [Hardware plan](docs/SENTINEL_CAMPUS_HARDWARE.md)
- [Live video and Triton setup](docs/SENTINEL_CAMPUS_LIVE_AND_TRITON.md)
- [Operator runbook](docs/SENTINEL_CAMPUS_RUNBOOK.md)
- [Product checklist](docs/SENTINEL_CAMPUS_PRODUCT_CHECKLIST.md)
- [Team FAQ](docs/SENTINEL_CAMPUS_TEAM_FAQ.md)
- [Leadership presentation](docs/SENTINEL_CAMPUS_PRESENTATION.html)

This public repository contains the Campus demo code only. It does not contain
camera credentials, local runtime state, model weights, or recording footage.
