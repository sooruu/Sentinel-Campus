module github.com/forensicspot/sentinel

go 1.25.0

// Pinned because 18 reachable standard-library vulnerabilities — including
// GO-2026-4599 and GO-2026-4601, both reached from internal/catalogue's HTTPS
// client — are fixed by 1.26.6 and present in 1.26.0. This is also OPS-12:
// building locally against a different stdlib than CI meant a local
// govulncheck result did not describe the shipped binary.
toolchain go1.26.6

require (
	github.com/ClickHouse/clickhouse-go/v2 v2.48.0
	github.com/bluenviron/gohlslib/v2 v2.4.3
	github.com/bluenviron/gortsplib/v5 v5.6.4
	github.com/bluenviron/mediacommon/v2 v2.9.3
	github.com/go-pdf/fpdf v0.9.0
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.3
	github.com/jackc/pgx/v5 v5.10.0
	github.com/pion/rtp v1.10.5
	golang.org/x/term v0.45.0
)

require (
	github.com/ClickHouse/ch-go v0.74.0 // indirect
	github.com/abema/go-mp4 v1.7.1 // indirect
	github.com/andybalholm/brotli v1.2.2 // indirect
	github.com/asticode/go-astikit v0.30.0 // indirect
	github.com/asticode/go-astits v1.16.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-faster/city v1.0.1 // indirect
	github.com/go-faster/errors v0.7.1 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/klauspost/compress v1.19.1 // indirect
	github.com/paulmach/orb v0.13.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.27 // indirect
	github.com/pion/logging v0.2.4 // indirect
	github.com/pion/randutil v0.1.0 // indirect
	github.com/pion/rtcp v1.2.17 // indirect
	github.com/pion/sdp/v3 v3.0.19 // indirect
	github.com/pion/srtp/v3 v3.0.13 // indirect
	github.com/pion/transport/v4 v4.1.0 // indirect
	github.com/segmentio/asm v1.2.1 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	go.opentelemetry.io/otel v1.44.0 // indirect
	go.opentelemetry.io/otel/trace v1.44.0 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
)
