---
id: "009"
title: "Enforce presigned URL expiry (X-Amz-Expires)"
status: closed
labels: [ready-for-agent]
assignee: claude
blocks: []
blocked-by: ["006"]
---

## What to build

Presigned URL validation (issue 006) verifies the SigV4 signature but never
checks `X-Amz-Expires` against wall-clock time — `X-Amz-Date` and
`X-Amz-Expires` are only signed-over data, not enforced. A validly-signed
presigned URL never expires.

- `internal/sigv4`: after `VerifyPresigned` confirms the signature, check
  that `X-Amz-Date` + `X-Amz-Expires` seconds has not elapsed relative to
  the current time.
- Decide and document the error for an expired-but-otherwise-valid
  signature (real S3 returns `AccessDenied` with a "Request has expired"
  message, distinct from `SignatureDoesNotMatch`).

## Acceptance criteria

- [x] A presigned URL whose `X-Amz-Date` + `X-Amz-Expires` window has
      passed is rejected, even with a valid signature.
- [x] A presigned URL still within its expiry window continues to succeed.
- [x] `docs/spec.md` §5.2/§7 updated to document the expiry check and its
      error code/status.

## Notes

Flagged during code review of issue 006. Not a regression — 006's
acceptance criteria never asked for expiry enforcement, and this is a
local dev tool rather than production S3, so it's plausible the answer
here is "no, punt" rather than implement. `needs-triage` until someone
decides whether this is worth the scope.

## Resolution

Implemented in `internal/sigv4.VerifyPresigned`: once `checkSignature`
confirms the signature, a new `checkExpiry` parses `X-Amz-Date` (layout
`20060102T150405Z`) and `X-Amz-Expires` (seconds) and compares
`signedAt + expires` against `time.Now()`. Both are covered by the
signature (they're part of the signed query string), so a tampered value
fails signature verification before expiry is even checked; a malformed but
untampered value (e.g. a non-numeric `X-Amz-Expires`) falls back to
`SignatureDoesNotMatch` for lack of a more specific code.

Chose `AccessDenied` / "Request has expired" for the expired-but-validly-signed
case, distinct from `SignatureDoesNotMatch`, matching real S3 behavior
mentioned in the issue notes — the handler (`internal/s3/handler.go`
`authorize`) now switches on `sigv4.ErrExpired` vs. other `VerifyPresigned`
errors. A malformed/missing `X-Amz-Date` or `X-Amz-Expires` still maps to
`SignatureDoesNotMatch`, consistent with how other structurally-invalid
presigned parameters were already handled.

`docs/spec.md` §5.2 and §7 updated. Test fixtures in `internal/sigv4` and
`internal/s3` switched from a hardcoded historical `X-Amz-Date` constant to
one computed at test run time (`time.Now()`), since a fixed past date would
now itself read as expired; new tests cover both the expired-rejection and
still-within-window-success paths.
