#!/usr/bin/env sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"

failures=0
check() {
	label=$1
	shift
	if "$@" >/dev/null 2>&1; then
		echo "PASS  $label"
	else
		echo "FAIL  $label"
		failures=$((failures + 1))
	fi
}

check "Go is installed" go version
check "Node is installed" node --version
check "npm is installed" npm --version
check "Campus frontend exists" test -f web-campus/package.json
check "Campus recordings are available" test -d "${SENTINEL_CAMPUS_RECORDINGS_DIR:-../Sentinel/recrodings}"

if [ -d web-campus/node_modules ]; then
	check "Campus frontend builds" sh -c 'cd web-campus && npm run build'
else
	echo "INFO  web-campus/node_modules is missing; run: make campus-build"
fi

if [ "$failures" -gt 0 ]; then
	echo "Campus check failed: $failures prerequisite(s) missing"
	exit 1
fi
echo "Campus check passed"
