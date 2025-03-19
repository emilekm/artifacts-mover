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

	state, err := internal.OpenState(stateFilename)
	if err != nil {
		return err
	}

	w, err := internal.NewWatcher(conf, state)
	if err != nil {
		return err
	}

	return w.Watch(ctx)
}
