package main

import (
	"log"

	"github.com/fsnotify/fsnotify"
)

const (
	configFilename = "config.yaml"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func run() error {
	config, err := newConfig(configFilename)
	if err != nil {
		return err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	return nil
}
