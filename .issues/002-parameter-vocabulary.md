---
id: "002"
title: "Full parameter vocabulary and validation"
status: closed
labels: [ready-for-agent]
assignee: claude
blocks: []
blocked-by: ["001"]
---

## What to build

Full support for the S3 key parameter vocabulary: a client can request any combination of `type`, `format`, `size`, `colour`, `text`, and `delay` segments and get back a correctly synthesized image, or a precise `InvalidArgument` error for bad input.

- `internal/key`: parse all segments per the grammar (`/`-separated `name=value`, `,`-separated multi-values, any order, percent-decoding); validate each value; return `InvalidArgument` naming the bad segment/value.
- `internal/image`: render `png` and `jpeg` in addition to `svg`; apply requested `size` and `colour` (hex or CSS named); draw `text` overlay with auto-contrasted colour; unconstrained text length.
- `internal/synth`/`internal/s3`: apply `delay` (fixed or random within an inclusive range) as a sleep before responding.
- `internal/s3`: serialize `InvalidArgument` (400) in the S3 XML error envelope for all validation failures, including unknown `type` values.

## Acceptance criteria

- [x] `/format=png/size=200x300/colour=ff0000/text=hello+world` returns a 200×300 PNG with red background and contrasting overlay text.
- [x] `/format=jpeg/size=400x200/colour=lightblue` returns a 400×200 JPEG with the named CSS colour.
- [x] `/delay=100,500` measurably delays the response within that range; `/delay=200` delays by exactly 200ms.
- [x] `/size=abc`, `/format=gif`, `/colour=notacolour`, and `/type=pdf` each return `InvalidArgument` 400 naming the bad parameter and value.
- [x] A key segment with no `=` separator returns `InvalidArgument` 400.

## Blocked by

- Walking skeleton: GetObject/HeadObject with default params
