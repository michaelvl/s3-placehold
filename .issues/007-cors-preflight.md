---
id: "007"
title: "CORS (always-on) and OPTIONS preflight"
status: closed
labels: [ready-for-agent]
assignee: claude
blocks: []
blocked-by: ["001"]
---

## What to build

Every response, including errors and preflight, carries permissive CORS headers, and `OPTIONS` preflight requests succeed even for private or unconfigured buckets.

- `internal/s3`: inject `Access-Control-Allow-Origin: *` and `Access-Control-Expose-Headers: ETag, Content-Type, Content-Length, x-amz-request-id` on every response.
- `internal/s3`: handle `OPTIONS` before bucket lookup and auth — respond `200 OK`, no body, with `Access-Control-Allow-Methods: GET, HEAD, DELETE, POST, OPTIONS`, `Access-Control-Allow-Headers: Authorization, Content-Type, x-amz-date, x-amz-content-sha256, x-amz-security-token, x-amz-user-agent`, and `Access-Control-Max-Age: 3600`, in addition to the headers above.

## Acceptance criteria

- [x] Every response (success, error, preflight) carries `Access-Control-Allow-Origin` and `Access-Control-Expose-Headers`.
- [x] `OPTIONS` against any bucket — including an unconfigured one or a private one with no credentials — returns `200` with the preflight headers and no body.

## Blocked by

- Walking skeleton: GetObject/HeadObject with default params
