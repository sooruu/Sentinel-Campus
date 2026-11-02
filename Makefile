.PHONY: campus campus-build campus-test campus-check

campus:
	./scripts/campus-demo.sh

campus-build:
	(cd web-campus && npm ci && npm run build)

campus-test:
	GOCACHE=$${TMPDIR:-/tmp}/sentinel-campus-go-cache go test ./campus ./cmd/campus-demo
	GOCACHE=$${TMPDIR:-/tmp}/sentinel-campus-go-cache go vet ./...

campus-check:
	./scripts/campus-check.sh
