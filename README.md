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
docker run --rm -p 9000:9000 \
  -e BUCKETS=images:public,assets:private \
  -e AWS_ACCESS_KEY_ID=demo \
  -e AWS_SECRET_ACCESS_KEY=demosecret \
  ghcr.io/michaelvl/s3-placehold/s3-placehold:latest
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
/colour=ff0000,00ff00,0000ff/gradient=mesh
```

| Segment  | Value syntax                                                             | Default   | Notes                                                                            |
| -------- | ------------------------------------------------------------------------ | --------- | -------------------------------------------------------------------------------- |
| `type`   | `image`                                                                  | `image`   | Routes to a synthesis pipeline. Only `image` exists today; unknown values → 400. |
| `format` | `svg` \| `png` \| `jpeg`                                                 | `svg`     | Output format and `Content-Type`. Other values → 400.                            |
| `size`   | `{width}x{height}`, e.g. `200x300`                                       | `DEFAULT_SIZE` (`100x100`) | Pixels. Non-integer or non-positive → 400.                            |
| `colour` | Up to 8 comma-separated values, each lowercase hex without `#` (`ff0000`) or a CSS named colour (`lightblue`) | `DEFAULT_COLOUR` (`cccccc`) | Background fill. Two or more colours are painted as a gradient. Unrecognised value, or more than 8 → 400. |
| `gradient` | `linear` with an optional angle (`linear:45`) \| `radial` \| `mesh` \| `none` | `DEFAULT_GRADIENT`, else `linear:90` for multi-colour keys and `none` otherwise | Geometry the `colour` list is painted with. One colour always paints flat, whatever this says. Other values → 400. |
| `text`   | URL-encoded string, `+` = space                                          | _(none)_  | Overlaid on the image; colour auto-contrasts against the background.             |
| `delay`  | Fixed ms (`200`) or an inclusive random range (`100,500`)                | `DEFAULT_DELAY_MS` | Server sleeps before responding, to simulate slow storage. Defaults to no delay unless `DEFAULT_DELAY_MS` is set; an explicit `delay` (including `delay=0`) overrides it. |

### Gradients

Two or more `colour` values are painted as a gradient, defaulting to a
left-to-right linear ramp. `gradient` selects the geometry:

- `linear:{deg}` — a straight ramp. The angle follows the CSS `linear-gradient`
  convention: `0` points towards the top and increases clockwise, so `90` is
  left-to-right. Angles outside `[0,360)` are wrapped.
- `radial` — a circle centred on the image, sized so the last colour reaches
  the corners. Note this is a circle, not the ellipse CSS `radial-gradient`
  defaults to, so on a very wide image the last colour only shows near the corners.
- `mesh` — the first colour fills the background and each remaining colour is a
  soft blob at a fixed position, giving colour that varies across both axes.
  Blob positions are fixed, not random, so a key always renders the same image.
- `none` — a flat fill of the first colour. Useful to override a configured
  `DEFAULT_GRADIENT`, in the same way `delay=0` overrides `DEFAULT_DELAY_MS`.

Geometry is chosen by the first of these that applies: an explicit `gradient`
segment, then `DEFAULT_GRADIENT`, then `linear:90` if the key has more than one
colour, then a flat fill. So `DEFAULT_GRADIENT=none` makes a multi-colour key
paint flat unless it names a `gradient` of its own.

All three formats render the same picture: `format` selects the encoding, not
the image. SVG output stays a few hundred bytes whatever the requested `size`,
since it is emitted as gradient definitions rather than pixels; PNG output of a
smooth gradient compresses well for the same reason.

## Configuration

All configuration is via environment variables:

| Variable                | Purpose                                                | Default              |
| ----------------------- | ------------------------------------------------------ | -------------------- |
| `PORT`                  | Listening port                                         | `9000`               |
| `BUCKETS`               | Comma-separated `name:mode` pairs (`public`/`private`) | `placeholder:public` |
| `AWS_ACCESS_KEY_ID`     | SigV4 access key (required if any bucket is `private`) | _(none)_             |
| `AWS_SECRET_ACCESS_KEY` | SigV4 secret key (required if any bucket is `private`) | _(none)_             |
| `MAX_X_PIXELS`          | Maximum allowed `size` width, in pixels                | `10000`              |
| `MAX_Y_PIXELS`          | Maximum allowed `size` height, in pixels               | `10000`              |
| `DEFAULT_SIZE`          | Size for keys with no `size` segment, as `{width}x{height}` | `100x100`       |
| `DEFAULT_COLOUR`        | Background fill for keys with no `colour` segment: up to 8 comma-separated hex values or CSS colour names | `cccccc` |
| `DEFAULT_GRADIENT`      | Gradient geometry for keys with no `gradient` segment: `linear[:deg]`, `radial`, `mesh` or `none` | _(none)_ |
| `DEFAULT_DELAY_MS`      | Delay for keys with no `delay` segment: fixed ms (`200`) or a range (`100,500`) | `0` (no delay) |

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
- **Large `mesh` images are slow to synthesize.** Mesh cost grows with
  width x height x colours; at the default `MAX_X_PIXELS`/`MAX_Y_PIXELS` of
  10000 it takes seconds. Lower the caps if that matters — a raster image that
  large is already expensive to encode regardless of the gradient.
