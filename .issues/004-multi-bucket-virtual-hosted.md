---
id: "004"
title: "Multi-bucket config, virtual-hosted URL style, and NoSuchBucket"
status: closed
labels: [ready-for-agent]
assignee: claude
blocks: ["005"]
blocked-by: ["001"]
---

## What to build

The server supports multiple configured buckets, each independently public or private, and can be addressed by either S3 URL style.

- `internal/config`: parse `BUCKETS` env var — comma-separated `name:public|private` pairs — into `[]BucketConfig`.
- `internal/s3`: detect URL style from the `Host` header on every request — virtual-hosted (`{bucket}.{server-host}`, bucket is first subdomain label, key is full path) vs. path-style (first path segment is bucket, remainder is key).
- `internal/s3`: look up the extracted bucket against configured buckets; unconfigured bucket → `NoSuchBucket` 404 S3 XML error.

## Acceptance criteria

- [x] `BUCKETS=images:public,assets:private` configures two buckets; requests to either resolve correctly by path style.
- [x] A request with `Host: images.localhost` and path `/format=png` resolves to bucket `images`, key `format=png` (virtual-hosted style).
- [x] A request to an unconfigured bucket name returns `NoSuchBucket` 404.

## Blocked by

- Walking skeleton: GetObject/HeadObject with default params
