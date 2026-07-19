---
id: "008"
title: "Container build (ko)"
status: closed
labels: [ready-for-agent]
assignee: claude
blocks: []
blocked-by: ["001"]
---

## What to build

The server builds as a distroless container image via `ko`, with version stamping and OCI annotations, no Dockerfile.

- `.ko.yaml` at repo root: `defaultBaseImage: gcr.io/distroless/static-debian12`; build id `s3-placehold`, main `./cmd/s3-placehold`, ldflags `-s -w -X main.version={{.Version}}`.
- Verify `ko build ./cmd/s3-placehold` produces a working image (`CGO_ENABLED=0` pure-Go binary).
- Document/apply OCI annotations: `org.opencontainers.image.source`, `org.opencontainers.image.description` (`S3-compatible placeholder image server`), `org.opencontainers.image.licenses` (`MIT`).
- Confirm the container exposes port 9000 by default, overridable via `PORT`.

## Acceptance criteria

- [x] `ko build ./cmd/s3-placehold` succeeds and produces a runnable image.
- [x] `docker run -p 9000:9000 <image>` starts the server with the zero-config default bucket, and the startup log line includes the stamped version.
- [x] The built image's `org.opencontainers.image.*` annotations match the spec.

## Blocked by

- Walking skeleton: GetObject/HeadObject with default params

## Resolution

Added `.ko.yaml` and a `make image` target (Makefile) wrapping `ko build --local` with the three `--image-annotation` flags.

One deviation from the spec text: `ldflags: -X main.version={{.Version}}` does not work — `ko`'s ldflags templating (verified against the installed `ko` 0.18.1 binary) only exposes `.Env` and `.Date`, not `.Version`/`.Commit`/etc. Used `-X main.version={{index .Env "VERSION"}}` instead (dot-access `.Env.VERSION` fails the build outright if `VERSION` is unset, so `index` is required for a bare `ko build` to still succeed). `make image` derives `VERSION` from `git describe --tags --always --dirty`. Verified end-to-end via `ko build --oci-layout-path` (no docker daemon available in this sandbox): image builds, manifest carries the three annotations, extracted binary logs `s3-placehold <version>` on start and serves the default bucket on the configured port. Documented in `docs/spec.md` §10.
