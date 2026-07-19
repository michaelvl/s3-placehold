// Command s3-placehold runs the S3-compatible placeholder image server.
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/michaelvl/s3-placehold/internal/config"
	"github.com/michaelvl/s3-placehold/internal/image"
	"github.com/michaelvl/s3-placehold/internal/s3"
	"github.com/michaelvl/s3-placehold/internal/synth"
)

// version is stamped at build time via -ldflags "-X main.version=...".
var version string

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	router := synth.NewRouter(image.New())
	handler := s3.NewHandler(cfg, router)

	log.Printf("s3-placehold %s", version)

	addr := fmt.Sprintf(":%d", cfg.Port)
	if err := http.ListenAndServe(addr, logRequests(handler)); err != nil {
		log.Fatal(err)
	}
}
