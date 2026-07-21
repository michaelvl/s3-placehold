VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null)

KO_DOCKER_REPO ?= ko.local
KO_TAGS ?= latest
IMAGE_SOURCE := https://github.com/michaelvl/s3-placehold
IMAGE_DESCRIPTION := S3-compatible placeholder image server
IMAGE_LICENSES := MIT

# Builds the s3-placehold binary into ./bin, stamped with the same VERSION
# used for the container image.
.PHONY: build
build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/s3-placehold ./cmd/s3-placehold

.PHONY: test
test:
	go test ./...

.PHONY: lint
lint:
	golangci-lint run

# Builds a distroless container image via ko and loads it into the local
# docker daemon, e.g. `docker run -p 9000:9000 ko.local/s3-placehold-...`.
.PHONY: image
image:
	VERSION=$(VERSION) KO_DOCKER_REPO=$(KO_DOCKER_REPO) ko publish --base-import-paths --tags="$(KO_TAGS)" \
		--image-annotation org.opencontainers.image.source=$(IMAGE_SOURCE) \
		--image-annotation org.opencontainers.image.description="$(IMAGE_DESCRIPTION)" \
		--image-annotation org.opencontainers.image.licenses=$(IMAGE_LICENSES) \
		./cmd/s3-placehold
