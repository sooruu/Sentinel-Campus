#!/usr/bin/env sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"

export SENTINEL_CAMPUS_HARDWARE=${SENTINEL_CAMPUS_HARDWARE:-simulator}
export SENTINEL_CAMPUS_PORT=${SENTINEL_CAMPUS_PORT:-8088}
export SENTINEL_CAMPUS_EVENTS_FILE=${SENTINEL_CAMPUS_EVENTS_FILE:-.sentinel-campus-events.json}
export SENTINEL_CAMPUS_REGISTRY_FILE=${SENTINEL_CAMPUS_REGISTRY_FILE:-.sentinel-campus-registry.json}
export SENTINEL_CAMPUS_LIVE_HLS_BASE=${SENTINEL_CAMPUS_LIVE_HLS_BASE:-http://127.0.0.1:18888}

if command -v npm >/dev/null 2>&1; then
	if [ ! -d web-campus/node_modules ]; then
		echo "Installing Campus console dependencies..."
		(cd web-campus && npm ci)
	fi
	echo "Building Campus console..."
	(cd web-campus && npm run build)
fi

echo "Sentinel Campus demo: http://localhost:${SENTINEL_CAMPUS_PORT}"
echo "Hardware backend: ${SENTINEL_CAMPUS_HARDWARE}"
echo "All gates start in SHADOW mode. Set SENTINEL_CAMPUS_AUTO_GATE=gate-1 only for the model-gate rehearsal."
exec go run ./cmd/campus-demo
