For local development and testing I want an S3-compatible object store that
serves placeholder resources, i.e. synthesises images from the URI instead of
serving existing media.

The path could be used to configure the syntesised media. We could use `/` to
separate 'arguments':

```text
/foo/bar # 'foo' and 'bar' are arguments
```

Arguments could have additional positional parameters separated by `-`:

```text
/foo-100/bar-200 # foo=100, bar=200
```

Examples S3 keys:

```text
/size-200x300/name-xyz/format-svg       # svg image with size 200x300 and name 'xyz'
/size-200x300/delay-100-500/format-png  # png image with size 200x300 served with random delay between 100-500ms
/size-200x300/colour-ffffff             # bacground colour 0xffffff
/size-200x300/text-hello+world          # text on image 'hello world' (spaces are represented with '+'
```
