VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null)

IMAGE_SOURCE := https://github.com/michaelvl/s3-placehold
IMAGE_DESCRIPTION := S3-compatible placeholder image server
IMAGE_LICENSES := MIT

# Builds a distroless container image via ko and loads it into the local
# docker daemon, e.g. `docker run -p 9000:9000 ko.local/s3-placehold-...`.
.PHONY: image
image:
	VERSION=$(VERSION) ko build --local \
		--image-annotation org.opencontainers.image.source=$(IMAGE_SOURCE) \
		--image-annotation org.opencontainers.image.description="$(IMAGE_DESCRIPTION)" \
		--image-annotation org.opencontainers.image.licenses=$(IMAGE_LICENSES) \
		./cmd/s3-placehold
