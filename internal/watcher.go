package internal

import (
	"context"
	"log/slog"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

type Watcher struct {
	handlers map[string]*Handler
}

func NewWatcher() *Watcher {
	return &Watcher{
		handlers: make(map[string]*Handler),
	}
}

func (w *Watcher) Register(paths []string, handler *Handler) {
	for _, path := range paths {
		path = filepath.Clean(path)
		w.handlers[path] = handler
	}
}

func (w *Watcher) Watch(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	defer watcher.Close()

	for path := range w.handlers {
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
				handler, ok := w.handlers[dir]
				if !ok {
					slog.Warn("No server found for file", slog.String("file", event.Name))
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
