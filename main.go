package main

import (
	"context"
	"log"
	"log/slog"
	"path/filepath"

	"github.com/emilekm/artifacts-mover/internal/config"
	"github.com/fsnotify/fsnotify"
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
	config, err := config.New(configFilename)
	if err != nil {
		return err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	defer watcher.Close()

	pathToServer := make(map[string]*Server)
	pathToType := make(map[string]ArtifactType)
	servers := make([]*Server, len(config.Servers))

	for i, svConf := range config.Servers {
		servers[i] = &Server{
			Config: svConf,
		}

		if err := watcher.Add(svConf.BF2Demos.Path); err != nil {
			return err
		}

		if err := watcher.Add(svConf.PRDemos.Path); err != nil {
			return err
		}

		if err := watcher.Add(svConf.Summaries.Path); err != nil {
			return err
		}

		pathToServer[svConf.BF2Demos.Path] = servers[i]
		pathToServer[svConf.PRDemos.Path] = servers[i]
		pathToServer[svConf.Summaries.Path] = servers[i]

		pathToType[svConf.BF2Demos.Path] = ArtifactTypeBF2Demo
		pathToType[svConf.PRDemos.Path] = ArtifactTypePRDemo
		pathToType[svConf.Summaries.Path] = ArtifactTypeSummary
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-watcher.Events:
			if event.Op.Has(fsnotify.Create) {
				dir := filepath.Dir(event.Name)
				sv, ok := pathToServer[dir]
				if !ok {
					slog.Warn("No server found for file", slog.String("file", event.Name))
					continue
				}

				typ, ok := pathToType[dir]
				if !ok {
					slog.Warn("No type found for file", slog.String("file", event.Name))
					continue
				}

				handleFile(sv, typ, event.Name)
			}
		case err := <-watcher.Errors:
			if err != nil {
				return err
			}
		}
	}
}

func handleFile(server *Server, typ ArtifactType, name string) {
	if server.CurrentRound == nil {
		server.CurrentRound = &Round{}
		server.Rounds = append(server.Rounds, server.CurrentRound)
	} else if typ == ArtifactTypeBF2Demo {
		queueForUpload(server, server.CurrentRound)
		server.CurrentRound = &Round{}
		server.Rounds = append(server.Rounds, server.CurrentRound)
	}

	switch typ {
	case ArtifactTypeBF2Demo:
		server.CurrentRound.BF2DemoFile = name
	case ArtifactTypePRDemo:
		server.CurrentRound.PRDemoFile = name
	case ArtifactTypeSummary:
		server.CurrentRound.SummaryFile = name
	}

	if isReadyForUpload(server.CurrentRound) {
		queueForUpload(server, server.CurrentRound)
	}
}

func isReadyForUpload(round *Round) bool {
	return round.PRDemoFile != "" &&
		round.BF2DemoFile != "" &&
		round.SummaryFile != ""
}

func queueForUpload(server *Server, round *Round) {
	if round.Uploaded {
		return
	}
}
