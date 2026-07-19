---
id: "006"
title: "Presigned URL support"
status: closed
labels: [ready-for-agent]
assignee: claude
blocks: []
blocked-by: ["005"]
---

## What to build

`GET`/`HEAD` requests carrying presigned-URL query parameters are validated and dispatched the same as regular GetObject/HeadObject requests.

- `internal/s3`: detect presigned requests via `X-Amz-Signature` query param; validate SigV4 signature using query params (`X-Amz-Signature`, `X-Amz-Credential`, `X-Amz-Date`, `X-Amz-Expires`) instead of the `Authorization` header, reusing the validation core from the private-bucket SigV4 work.
- `internal/s3`: on valid signature, dispatch as GetObject or HeadObject per the method; on invalid signature, `SignatureDoesNotMatch` 403.

## Acceptance criteria

- [x] A validly presigned `GET` URL returns the synthesized image as if it were a normal GetObject request.
- [x] A validly presigned `HEAD` URL returns headers only, no body.
- [x] A presigned URL with an invalid signature returns `SignatureDoesNotMatch` 403.

## Blocked by

- Private bucket auth: SigV4 validation (header-based)
