package main

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/emilekm/artifacts-mover/internal"
	"github.com/emilekm/artifacts-mover/internal/config"
)

const (
	configFilename = "config.yaml"
)

func main() {
	ctx := context.Background()
	if err := run(ctx); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func run(ctx context.Context) error {
	conf, err := config.New(configFilename)
	if err != nil {
		return err
	}

	failedPath := conf.FailedUploadPath
	if failedPath == "" {
		failedPath = "./failed"
	}

	w := internal.NewWatcher()
	q := internal.NewQueue()

	for name, server := range conf.Servers {
		svFailedPath := filepath.Join(failedPath, name)
		if err := os.MkdirAll(svFailedPath, 0755); err != nil {
			return err
		}

		uploader, err := internal.NewUploader(q, server.Upload, server.Artifacts, svFailedPath)
		if err != nil {
			return err
		}

		handler := internal.NewHandler(uploader, server.Artifacts)

		paths := make([]string, 0, len(server.Artifacts))

		for _, loc := range server.Artifacts {
			paths = append(paths, loc.Location)
		}

		w.Register(paths, handler)
	}

	return w.Watch(ctx)
}
