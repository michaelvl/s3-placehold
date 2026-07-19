---
id: "003"
title: "ListObjects/V2 and Delete/DeleteObjects no-ops"
status: closed
labels: [ready-for-agent]
assignee: claude
blocks: []
blocked-by: ["001"]
---

## What to build

The non-synthesis S3 operations respond correctly without doing any real work: listing always reports empty, and deletes are silent no-ops.

- `internal/s3`: dispatch `GET` requests carrying `list-type` or any listing param (`prefix`, `delimiter`, `marker`, `continuation-token`) to ListObjects/V2, returning a valid empty S3 list XML response.
- `internal/s3`: dispatch `DELETE` (no query params) to DeleteObject, returning `204 No Content`.
- `internal/s3`: dispatch `POST` with a `delete` query param to DeleteObjects (batch), returning `200 OK` with an empty `DeleteResult` XML body.

## Acceptance criteria

- [x] `curl http://localhost:9000/placeholder/?list-type=2` returns `200` with a valid empty `ListBucketResult` XML (zero keys).
- [x] `curl -X DELETE http://localhost:9000/placeholder/some/key` returns `204` with no body.
- [x] `curl -X POST http://localhost:9000/placeholder/?delete` returns `200` with an empty `DeleteResult` XML body.

## Blocked by

- Walking skeleton: GetObject/HeadObject with default params
