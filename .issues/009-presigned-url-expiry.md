---
id: "009"
title: "Enforce presigned URL expiry (X-Amz-Expires)"
status: open
labels: [needs-triage]
assignee: null
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

- [ ] A presigned URL whose `X-Amz-Date` + `X-Amz-Expires` window has
      passed is rejected, even with a valid signature.
- [ ] A presigned URL still within its expiry window continues to succeed.
- [ ] `docs/spec.md` §5.2/§7 updated to document the expiry check and its
      error code/status.

## Notes

Flagged during code review of issue 006. Not a regression — 006's
acceptance criteria never asked for expiry enforcement, and this is a
local dev tool rather than production S3, so it's plausible the answer
here is "no, punt" rather than implement. `needs-triage` until someone
decides whether this is worth the scope.
