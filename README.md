# s3-placehold

An S3-compatible placeholder image server. Instead of storing objects, it
**synthesizes** an image on demand from parameters encoded in the request key —
no bucket to pre-populate, no fixtures to check in. Point any S3 client or SDK
at it and ask for `size=300x200/format=png/colour=lightblue` and get back a real
PNG of that size and colour, generated on the fly.

It's meant for local development and testing: seeding UI screenshots, exercising
image-loading code paths, or standing in for a real S3/CDN endpoint in a
docker-compose stack.

## Quick start

```sh
make image
docker run --rm -p 9000:9000 \
  -e BUCKETS=images:public,assets:private \
  -e AWS_ACCESS_KEY_ID=demo \
  -e AWS_SECRET_ACCESS_KEY=demosecret \
  ghcr.io/michaelvl/s3-placehold:latest
```

This configures a public `images` bucket and a private `assets` bucket; every
example below runs against this one container. (With no `BUCKETS`/credentials
set at all, the server instead defaults to a single public bucket named
`placeholder` on port `9000`.)

Fetch a placeholder image from the public bucket (path-style:
`/{bucket}/{key}`):

```sh
curl http://localhost:9000/images/size=300x200/format=png/colour=lightblue -o out.png
```

### More GetObject / HeadObject examples

```sh
# Defaults: 100x100 svg, #cccccc background
curl http://localhost:9000/images/

# Text overlay ("+" is a space)
curl http://localhost:9000/images/format=png/size=400x200/text=hello+world -o out.png

# Simulated latency: fixed 200ms, or a random 100-500ms
curl http://localhost:9000/images/delay=200
curl http://localhost:9000/images/delay=100,500

# HeadObject: same synthesis, headers only, no body
curl -I http://localhost:9000/images/format=png/size=200x300
```

Virtual-hosted style (`{bucket}.{host}`) works too — resolve the bucket
subdomain to the server, or fake it with curl's `--resolve`/`Host` header:

```sh
curl --resolve images.localhost:9000:127.0.0.1 \
  http://images.localhost:9000/format=png
```

### Listing and delete (no-ops)

`ListObjects`/`ListObjectsV2`, `DeleteObject`, and batch `DeleteObjects` are
accepted and return well-formed but empty/no-op responses — nothing is ever
actually stored, so there's nothing to list or delete:

```sh
curl "http://localhost:9000/images/?list-type=2"
curl -X DELETE http://localhost:9000/images/format=png
curl -X POST "http://localhost:9000/images/?delete"
```

### Private buckets and presigned URLs

A `private` bucket requires a valid AWS SigV4 signature — either as an
`Authorization` header or as a presigned URL's query parameters. Hand-rolling a
SigV4 signature isn't practical with plain `curl`; use the AWS CLI or any AWS
SDK pointed at the server as a custom endpoint. The `assets` bucket configured
above is `private`:

```sh
export AWS_ACCESS_KEY_ID=demo
export AWS_SECRET_ACCESS_KEY=demosecret

aws --endpoint-url http://localhost:9000 s3api get-object \
  --bucket assets --key format=png/size=300x300 out.png

# Presigned URL (rejected once its expiry window elapses)
aws --endpoint-url http://localhost:9000 s3 presign \
  s3://assets/format=png/size=300x300 --expires-in 300
```

## Parameters

Every request key is a sequence of `/`-separated `name=value` segments, in any
order, all optional:

```
/format=png/size=200x300/colour=ff0000/text=hello+world
```

| Segment  | Value syntax                                                             | Default   | Notes                                                                            |
| -------- | ------------------------------------------------------------------------ | --------- | -------------------------------------------------------------------------------- |
| `type`   | `image`                                                                  | `image`   | Routes to a synthesis pipeline. Only `image` exists today; unknown values → 400. |
| `format` | `svg` \| `png` \| `jpeg`                                                 | `svg`     | Output format and `Content-Type`. Other values → 400.                            |
| `size`   | `{width}x{height}`, e.g. `200x300`                                       | `100x100` | Pixels. Non-integer or non-positive → 400.                                       |
| `colour` | Lowercase hex without `#` (`ff0000`) or a CSS named colour (`lightblue`) | `cccccc`  | Background fill. Unrecognised value → 400.                                       |
| `text`   | URL-encoded string, `+` = space                                          | _(none)_  | Overlaid on the image; colour auto-contrasts against the background.             |
| `delay`  | Fixed ms (`200`) or an inclusive random range (`100,500`)                | _(none)_  | Server sleeps before responding, to simulate slow storage.                       |

## Configuration

All configuration is via environment variables:

| Variable                | Purpose                                                | Default              |
| ----------------------- | ------------------------------------------------------ | -------------------- |
| `PORT`                  | Listening port                                         | `9000`               |
| `BUCKETS`               | Comma-separated `name:mode` pairs (`public`/`private`) | `placeholder:public` |
| `AWS_ACCESS_KEY_ID`     | SigV4 access key (required if any bucket is `private`) | _(none)_             |
| `AWS_SECRET_ACCESS_KEY` | SigV4 secret key (required if any bucket is `private`) | _(none)_             |

## Key limitations vs. real AWS S3

- **Nothing is stored.** Every `GetObject`/`HeadObject` is synthesized fresh
  from the key; there's no bucket contents, no persistence, no ETags tied to
  real object state.
- **Listing is always empty.** `ListObjects`/`ListObjectsV2` return a valid but
  empty result regardless of what's been "written".
- **Writes and deletes are no-ops.** `PUT`/`DeleteObject`/`DeleteObjects` don't
  fail, but nothing happens — good enough to unblock a client's cleanup code,
  not to test it.
- **Single credential pair, no IAM.** One access/secret key validates SigV4
  signatures; there's no per-bucket policy, ACLs, or multi-user auth.
- **CORS is always permissive** (`Access-Control-Allow-Origin: *`) — there's no
  per-bucket CORS configuration.
- **No multipart upload, versioning, or object metadata** beyond the
  `Content-Type`/`Content-Length` implied by the key.
