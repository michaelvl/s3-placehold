---
id: "005"
title: "Private bucket auth: SigV4 validation (header-based)"
status: closed
labels: [ready-for-agent]
assignee: claude
blocks: ["006"]
blocked-by: ["004"]
---

## What to build

Requests to a `private` bucket are rejected unless they carry a valid AWS SigV4 signature in the `Authorization` header, validated against the single configured credential pair.

- `internal/s3` (or a new `internal/sigv4` used by it): validate the `Authorization` header's SigV4 signature against `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY` from config.
- `internal/s3`: a request to a `private` bucket with no credentials → `AccessDenied` 403; a present-but-invalid signature → `SignatureDoesNotMatch` 403.
- `internal/s3`: a request to a `public` bucket is never subject to this check, regardless of credentials present.

## Acceptance criteria

- [x] A correctly SigV4-signed request to a private bucket succeeds and dispatches normally.
- [x] An unsigned request to a private bucket returns `AccessDenied` 403.
- [x] A signed request with a tampered signature returns `SignatureDoesNotMatch` 403.
- [x] Requests to a public bucket succeed with or without credentials.

## Blocked by

- Multi-bucket config, virtual-hosted URL style, and NoSuchBucket
