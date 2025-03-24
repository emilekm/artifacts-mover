package internal

import (
	"context"
	"log/slog"
	"path/filepath"

	"github.com/emilekm/artifacts-mover/internal/config"
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
	err := w.handleOldFiles()
	if err != nil {
		return err
	}

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

func (w *Watcher) handleOldFiles() error {
	handlers := make(map[*Handler][][]string)

	for path, handler := range w.handlers {
		if _, ok := handlers[handler]; !ok {
			handlers[handler] = make([][]string, 0)
		}

		files, err := filepath.Glob(filepath.Join(path, "*"))
		if err != nil {
			return err
		}

		handlers[handler] = append(handlers[handler], files)
	}

	// Ensure that the bf2Demo files are processed first
	for handler, allFiles := range handlers {
		if len(allFiles) > 1 {
			if len(allFiles[0]) > 0 {
				typ := handler.LocToType[filepath.Dir(allFiles[0][0])]
				if typ != config.ArtifactTypeBF2Demo {
					replaced := false
					for i, files := range allFiles[1:] {
						if len(files) > 0 {
							typ2 := handler.LocToType[filepath.Dir(allFiles[0][0])]
							if typ2 == config.ArtifactTypeBF2Demo {
								allFiles[0], allFiles[i+1] = allFiles[i+1], allFiles[0]
								replaced = true
								break
							}
						}
					}
					if replaced {
						break
					}
				}
			}
		}
	}

	for handler, allFiles := range handlers {
		// The number of files in each directory should be the same
		// or the first directory should have more files than the others
		for i := range allFiles[0] {
			for _, files := range allFiles {
				if len(files) > i {
					handler.OnFileCreate(files[i])
				}
			}
		}
	}

	return nil
}
