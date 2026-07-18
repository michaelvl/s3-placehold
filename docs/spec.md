# s3-placehold — Implementation Spec

An S3-compatible placeholder image server that synthesizes images from structured S3 key paths, served as a container. This document is the authoritative spec for a from-scratch Go implementation.

---

## 1. Purpose

`s3-placehold` is a drop-in S3 endpoint for local development and testing. Instead of storing objects, it synthesizes images on demand from parameters encoded in the S3 key. Any S3 client pointed at it can request a placeholder image of any size, colour, and format by constructing the appropriate key — no pre-populated bucket required.

---

## 2. S3 Key Schema

An S3 key is a sequence of `/`-separated segments in any order. Each segment is `name=value`. Multiple values within a single segment are `,`-separated (used for ranges). There is no filename suffix.

**Grammar:**

```
key      = "/" segment ("/" segment)*
segment  = name "=" value ("," value)*
name     = [a-z]+
value    = [^/,=]+
```

**Example keys:**

```
/format=svg/size=200x300/colour=ff0000/text=hello+world
/size=400x200/colour=lightblue/delay=100,500/format=png
/type=image/format=jpeg
/size=800x600
```

**Encoding:** `=` and `,` are valid S3 key characters. AWS SDKs percent-encode them in URLs; the server decodes percent-encoding transparently before parsing.

**All segments are optional.** Missing segments fall back to their defaults (see §3).

**Extensibility:** a future `type=json` segment routes to a different synthesis pipeline; no change to the schema is required.

---

## 3. Parameter Vocabulary

| Segment name | Value syntax | Default | Notes |
|---|---|---|---|
| `type` | `image` | `image` | Routes to synthesis pipeline. Only `image` is in scope for this spec. Unknown values → `InvalidArgument` 400. |
| `format` | `svg` \| `png` \| `jpeg` | `svg` | Output format for `type=image`. Other values → `InvalidArgument` 400. |
| `size` | `{W}x{H}` e.g. `200x300` | `100x100` | Width × height in pixels. Non-integer or non-positive values → `InvalidArgument` 400. |
| `colour` | Lowercase hex without `#` e.g. `ff0000`, **or** a single-word CSS named colour e.g. `lightblue` | `cccccc` | Background fill colour. Unrecognised value → `InvalidArgument` 400. |
| `text` | URL-encoded string; `+` represents a space | *(no overlay)* | Text drawn on the image. Unconstrained length; colour is auto-contrasted against the background. |
| `delay` | Fixed ms: `200`; random range: `100,500` | *(no delay)* | Server sleeps before responding. Unit: milliseconds. Two comma-separated values are treated as an inclusive range from which a random duration is drawn. |

---

## 4. S3 API Surface

### 4.1 Supported operations

| Operation | Behaviour |
|---|---|
| `GetObject` | Synthesize image from key; return bytes with correct `Content-Type` and `Content-Length`. |
| `HeadObject` | Synthesize full image (for exact sizing), return `Content-Type` and `Content-Length` headers, **no body**. |
| `ListObjects` / `ListObjectsV2` | Return a valid empty S3 list response (zero keys). |
| `DeleteObject` | Silent no-op. Return `204 No Content`. |
| `DeleteObjects` (batch) | Silent no-op. Return `200 OK` with an empty `DeleteResult` XML body. |
| Presigned `GET`/`HEAD` | Validate SigV4 signature against the configured credential pair, then dispatch as GetObject or HeadObject above. |
| `OPTIONS` | CORS preflight — respond 200 with CORS headers, no body. Handled before auth and bucket lookup. |

All other HTTP methods (e.g. `PUT`, `PATCH`) return `MethodNotAllowed` 405.

### 4.2 Operation dispatch

After extracting bucket and key (see §5), dispatch on method and query parameters:

| Method | Query parameters present | Dispatched operation |
|---|---|---|
| `GET` | none | GetObject |
| `HEAD` | none | HeadObject |
| `GET` | `list-type` or any listing param (`prefix`, `delimiter`, `marker`, `continuation-token`) | ListObjects/V2 |
| `DELETE` | none | DeleteObject |
| `POST` | `delete` | DeleteObjects |
| `GET` or `HEAD` | `X-Amz-Signature` | Presigned — validate SigV4, then dispatch as above |
| `OPTIONS` | any | CORS preflight — respond 200 with CORS preflight headers, no body |

### 4.3 URL styles

Both S3 URL styles are supported. The server detects style by inspecting the `Host` header on every request:

- **Virtual-hosted style:** `Host: {bucket}.{server-host}` — bucket is the first subdomain label; key is the full request path.
- **Path style:** any other `Host` value — first path segment is the bucket name; the remainder of the path is the key.

Path style avoids DNS wildcard requirements in local Docker Compose setups and is the recommended default for local development.

### 4.4 Content-Type mapping

| Format | MIME type |
|---|---|
| `svg` | `image/svg+xml` |
| `png` | `image/png` |
| `jpeg` | `image/jpeg` |

---

## 5. Authentication

### 5.1 Per-bucket auth mode

Each bucket is configured as either `public` or `private` (see §8). For a **public** bucket, all requests are accepted without credentials. For a **private** bucket, every request must carry a valid AWS SigV4 signature.

### 5.2 SigV4 validation

The server validates SigV4 signatures against a single configured credential pair (`AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY`). It does not need to generate signatures — only verify them.

Presigned URLs carry the signature in query parameters (`X-Amz-Signature`, `X-Amz-Credential`, `X-Amz-Date`, `X-Amz-Expires`). Regular authenticated requests carry the `Authorization` header.

Invalid or missing credentials on a private bucket return `AccessDenied` or `SignatureDoesNotMatch` as appropriate (see §7).

---

## 6. CORS

CORS is always-on and permissive. No configuration is required or provided — this is a dev tool, and cross-origin access is the normal case.

### 6.1 Headers on every response

These headers are added to **all** responses, including errors and preflight:

| Header | Value |
|---|---|
| `Access-Control-Allow-Origin` | `*` |
| `Access-Control-Expose-Headers` | `ETag, Content-Type, Content-Length, x-amz-request-id` |

### 6.2 OPTIONS preflight

`OPTIONS` requests are handled **before** bucket lookup and auth so that preflight succeeds even for private or unconfigured buckets. The response is `200 OK` with no body and the following headers (in addition to those in §6.1):

| Header | Value |
|---|---|
| `Access-Control-Allow-Methods` | `GET, HEAD, DELETE, POST, OPTIONS` |
| `Access-Control-Allow-Headers` | `Authorization, Content-Type, x-amz-date, x-amz-content-sha256, x-amz-security-token, x-amz-user-agent` |
| `Access-Control-Max-Age` | `3600` |

---

## 7. Error Handling

All errors are returned in the standard S3 XML envelope:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InvalidArgument</Code>
  <Message>Invalid value for parameter 'size': 'abc'</Message>
  <RequestId>550e8400-e29b-41d4-a716-446655440000</RequestId>
</Error>
```

`Content-Type` on error responses is `application/xml`.

`RequestId` is a random UUID generated per request.

`Message` names the specific bad segment or parameter and its value where applicable.

### Error table

| Condition | `<Code>` | HTTP status |
|---|---|---|
| Key segment has no `=` separator | `InvalidArgument` | 400 |
| Parameter value is invalid (e.g. `size=abc`, `format=gif`, `colour=notacolour`) | `InvalidArgument` | 400 |
| Unknown `type` value (e.g. `type=pdf`) | `InvalidArgument` | 400 |
| Bucket not in the configured bucket list | `NoSuchBucket` | 404 |
| Request to a private bucket with no credentials | `AccessDenied` | 403 |
| SigV4 signature present but invalid | `SignatureDoesNotMatch` | 403 |
| Presigned URL signature invalid | `SignatureDoesNotMatch` | 403 |
| Unsupported HTTP method | `MethodNotAllowed` | 405 |

---

## 8. Server Configuration

All configuration is via environment variables. No config file, no flags.

| Variable | Purpose | Default |
|---|---|---|
| `PORT` | Listening port | `9000` |
| `BUCKETS` | Comma-separated `name:mode` pairs | `placeholder:public` |
| `AWS_ACCESS_KEY_ID` | SigV4 access key (required when any bucket is `private`) | *(none)* |
| `AWS_SECRET_ACCESS_KEY` | SigV4 secret key (required when any bucket is `private`) | *(none)* |

**`BUCKETS` format:** each entry is `name:public` or `name:private`. Multiple buckets are comma-separated.

```
BUCKETS=images:public,assets:private
```

**Zero-config default:** `docker run -p 9000:9000 s3-placehold` starts with a single `placeholder` bucket in public mode on port 9000.

---

## 9. Go Project Structure

### 8.1 Directory layout

```
s3-placehold/
├── cmd/
│   └── s3-placehold/
│       └── main.go          # wires config → synth → handler; calls ListenAndServe
├── internal/
│   ├── config/
│   │   └── config.go        # parse env vars → Config struct
│   ├── s3/
│   │   └── handler.go       # HTTP handlers, routing, SigV4 validation, XML serialization
│   ├── key/
│   │   └── key.go           # parse S3 key string → Params struct; validate values
│   ├── synth/
│   │   └── synth.go         # Synthesizer interface + type dispatch
│   └── image/
│       └── image.go         # Synthesizer implementation for type=image
└── .ko.yaml
```

### 8.2 Package responsibilities

| Package | Responsibility |
|---|---|
| `internal/config` | Parse env vars into a `Config` struct with fields `Port int`, `Buckets []BucketConfig`, `AccessKeyID string`, `SecretAccessKey string`. |
| `internal/key` | Parse an S3 key string into a typed `Params` struct; validate all parameter values; return `InvalidArgument` errors for bad input. |
| `internal/synth` | Define the `Synthesizer` interface; dispatch on `Params.Type` to the right backend. |
| `internal/image` | Implement `Synthesizer` for `type=image`; produce raw image bytes in the requested format. Implementer chooses the image synthesis library. |
| `internal/s3` | HTTP handlers; CORS header injection on every response; OPTIONS preflight handling (before auth); virtual-hosted vs. path-style URL detection; method+query-param operation dispatch; SigV4 validation; S3 XML response and error serialization. |

### 8.3 Synthesizer interface

Defined in `internal/synth`:

```go
type Synthesizer interface {
    Synthesize(params key.Params) (data []byte, mimeType string, err error)
}
```

`len(data)` is used as the exact `Content-Length` for both GetObject and HeadObject. Adding a future `type=pdf` backend means implementing this interface in a new package — `internal/s3` is untouched.

### 8.4 Key Params struct

Defined in `internal/key`:

```go
type Params struct {
    Type    string        // default "image"
    Format  string        // default "svg"
    Width   int           // default 100
    Height  int           // default 100
    Colour  color.RGBA    // default #cccccc
    Text    string        // empty means no overlay
    DelayMin time.Duration // 0 means no delay
    DelayMax time.Duration // equal to DelayMin for fixed delay; > DelayMin for range
}
```

### 8.5 Request pipeline

Per request, execution flows:

1. **`internal/s3`** — set CORS headers on the response writer; if `OPTIONS`, respond immediately with preflight headers and return.
2. **`internal/s3`** — extract bucket + key from URL; look up bucket config; check auth.
3. **`internal/s3`** — dispatch operation (GetObject, HeadObject, ListObjects, Delete*, presigned).
4. **`internal/key`** — parse key string → `Params`; return `InvalidArgument` on bad input.
5. **`internal/synth`** — call `Synthesizer.Synthesize(params)`; apply delay.
6. **`internal/s3`** — write response: bytes + headers for GetObject; headers only for HeadObject; XML for errors and list/delete operations.

### 8.6 HTTP router

Use stdlib `net/http` only — no third-party router. A single `Handler` type in `internal/s3` satisfies `http.Handler`. All dispatch logic lives within that type.

### 8.7 main.go

`cmd/s3-placehold/main.go` must:

1. Declare `var version string` (stamped by ko at build time).
2. Call `config.Load()` to parse env vars.
3. Construct the `Synthesizer`.
4. Construct the `s3.Handler` with config and synthesizer.
5. Log the version at startup: `log.Printf("s3-placehold %s", version)`.
6. Call `http.ListenAndServe`.

---

## 10. Container Build

There is no Dockerfile. The image is built with [`ko`](https://ko.build).

**Base image:** `gcr.io/distroless/static-debian12` — no shell, includes CA certs and `nobody` user; compatible with a pure-Go binary built with `CGO_ENABLED=0`.

**Build command:**

```sh
ko build ./cmd/s3-placehold
```

**`.ko.yaml`** at repo root:

```yaml
defaultBaseImage: gcr.io/distroless/static-debian12

builds:
  - id: s3-placehold
    main: ./cmd/s3-placehold
    ldflags:
      - -s
      - -w
      - -X main.version={{.Version}}
```

**OCI annotations** — set via `ko build --image-annotation` flags or equivalent in the build pipeline:

| Annotation | Value |
|---|---|
| `org.opencontainers.image.source` | Repository URL |
| `org.opencontainers.image.description` | `S3-compatible placeholder image server` |
| `org.opencontainers.image.licenses` | `MIT` |

**Exposed port:** 9000 (overridable via `PORT` env var).

---

## 11. Out of Scope

The following are explicitly not part of this spec:

- Non-image synthesis (`type=json`, PDF, etc.) — the schema and `Synthesizer` interface support it, but no implementation is required here.
- Actual object persistence or storage.
- Richer `ListObjects` enumeration — the server always returns an empty list.
- Authentication *enforcement* beyond SigV4 signature validation (no IAM, no role policies).
