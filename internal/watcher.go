package internal

import (
	"context"
	"log/slog"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

type fileHandler interface {
	OnFileCreate(path string)
}

type Watcher struct {
	handlers map[string]fileHandler
}

func NewWatcher() *Watcher {
	return &Watcher{
		handlers: make(map[string]fileHandler),
	}
}

func (w *Watcher) Register(paths []string, handler fileHandler) {
	for _, path := range paths {
		path = filepath.Clean(path)
		w.handlers[path] = handler
	}
}

func (w *Watcher) Watch(ctx context.Context) error {
	log := slog.With("op", "Watcher.Watch")

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	defer watcher.Close()

	for path := range w.handlers {
		log.Debug("Adding path to watcher", "path", path)
		if err := watcher.Add(path); err != nil {
			return err
		}
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-watcher.Events:
			log.Debug("Received file event", "event", event.Op, "path", event.Name)
			if event.Op.Has(fsnotify.Create) {
				dir := filepath.Dir(event.Name)
				handler, ok := w.handlers[dir]
				if !ok {
					log.Warn("No server found for file", slog.String("file", event.Name))
					continue
				}

				handler.OnFileCreate(event.Name)
			}
		case err := <-watcher.Errors:
			if err != nil {
				return err
			}
		}
	}
}
