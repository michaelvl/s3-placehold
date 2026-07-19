---
id: "001"
title: "Walking skeleton: GetObject/HeadObject with default params"
status: closed
labels: [ready-for-agent]
assignee: claude
blocks: ["002", "003", "004", "007", "008"]
blocked-by: []
---

## What to build

A minimal end-to-end path: an S3 client can `GetObject`/`HeadObject` against the default `placeholder` bucket with no key parameters and receive a synthesized default placeholder image.

- `internal/config`: `config.Load()` parses env vars into a `Config` struct; zero-config default is a single `placeholder` bucket in `public` mode on port `9000`.
- `internal/key`: parse an empty/default key string into a `Params` struct using only defaults (`type=image`, `format=svg`, `size=100x100`, `colour=cccccc`, no text, no delay).
- `internal/image`: `Synthesizer` implementation producing a solid-colour SVG for the default params.
- `internal/synth`: `Synthesizer` interface, dispatching `Params.Type` to the image backend.
- `internal/s3`: `Handler` satisfying `http.Handler`; path-style bucket/key extraction; dispatch `GET` (no query params) → GetObject, `HEAD` (no query params) → HeadObject; any other method → `MethodNotAllowed` 405 S3 XML error.
- `cmd/s3-placehold/main.go`: declares `var version string`, calls `config.Load()`, constructs synthesizer + handler, logs version, calls `http.ListenAndServe`.

## Acceptance criteria

- [x] `go run ./cmd/s3-placehold` starts a server on port 9000 with the zero-config default bucket.
- [x] `curl http://localhost:9000/placeholder/` returns a 100×100 `#cccccc` SVG with `Content-Type: image/svg+xml` and correct `Content-Length`.
- [x] `curl -I http://localhost:9000/placeholder/` returns the same headers with no body.
- [x] `curl -X PUT http://localhost:9000/placeholder/` returns `405 MethodNotAllowed` in the S3 XML error envelope with a random `RequestId`.

## Blocked by

None — can start immediately.
