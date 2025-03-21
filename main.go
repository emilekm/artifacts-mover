package main

import (
	"context"
	"log"

	"github.com/emilekm/artifacts-mover/internal"
	"github.com/emilekm/artifacts-mover/internal/config"
)

const (
	configFilename = "config.yaml"
	stateFilename  = "state.json"
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

	w := internal.NewWatcher()
	q := internal.NewQueue()

	for _, server := range conf.Servers {
		uploader, err := internal.NewUploader(q, server.Upload, server.Artifacts)
		if err != nil {
			return err
		}

		handler := internal.NewHandler(server.Artifacts, uploader)

		paths := make([]string, 0, len(server.Artifacts))

		for _, loc := range server.Artifacts {
			paths = append(paths, loc.Directory)
		}

		w.Register(paths, handler)
	}

	return w.Watch(ctx)
}
