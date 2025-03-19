package internal

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/emilekm/artifacts-mover/internal/config"
	"github.com/fsnotify/fsnotify"
)

type pathInfo struct {
	Type   config.ArtifactType
	Server *Server
}

type Watcher struct {
	queue      *Queue
	pathToInfo map[string]pathInfo
	state      *State
}

func NewWatcher(config *config.Config, state *State) (*Watcher, error) {
	w := &Watcher{
		queue:      &Queue{},
		pathToInfo: make(map[string]pathInfo),
		state:      state,
	}

	for _, svConf := range config.Servers {
		sv := &Server{
			Config: svConf,
		}

		for typ, loc := range svConf.Artifacts {
			w.pathToInfo[filepath.Clean(loc.Directory)] = pathInfo{
				Type:   typ,
				Server: sv,
			}
		}

		fmt.Printf("%+v\n", w.pathToInfo)
	}

	return w, nil
}

func (w *Watcher) Watch(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	defer watcher.Close()

	for path := range w.pathToInfo {
		if err := watcher.Add(path); err != nil {
			return err
		}
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-watcher.Events:
			if event.Op.Has(fsnotify.Create) {
				dir := filepath.Dir(event.Name)
				info, ok := w.pathToInfo[dir]
				if !ok {
					slog.Warn("No server found for file", slog.String("file", event.Name))
					continue
				}

				w.handleFile(info.Server, info.Type, event.Name)
			}
		case err := <-watcher.Errors:
			if err != nil {
				return err
			}
		}
	}
}

func (w *Watcher) handleFile(server *Server, typ config.ArtifactType, path string) {
	slog.Info("New file", slog.String("file", path), slog.String("type", typ.String()))
	if server.CurrentRound == nil {
		server.CurrentRound = NewRound()
		server.Rounds = append(server.Rounds, server.CurrentRound)
	} else if typ == config.ArtifactTypeBF2Demo {
		w.queueForUpload(server, server.CurrentRound)
		server.CurrentRound = NewRound()
		server.Rounds = append(server.Rounds, server.CurrentRound)
	}

	server.CurrentRound.Files[typ] = path

	if isReadyForUpload(server.Config, server.CurrentRound) {
		w.queueForUpload(server, server.CurrentRound)
	}
}

func (w *Watcher) queueForUpload(server *Server, round *Round) {
	if round.Uploaded {
		return
	}

	w.queue.Add(server, round)
}

func isReadyForUpload(conf *config.Server, round *Round) bool {
	return len(round.Files) == len(conf.Artifacts)
}
