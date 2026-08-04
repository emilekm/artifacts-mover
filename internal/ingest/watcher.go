package ingest

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
	logger   *slog.Logger
	handlers map[string]fileHandler
}

func NewWatcher(logger *slog.Logger) *Watcher {
	return &Watcher{
		logger:   logger,
		handlers: make(map[string]fileHandler),
	}
}

func (w *Watcher) Register(paths []string, handler fileHandler) {
	for _, path := range paths {
		path = filepath.Clean(path)
		w.handlers[path] = handler
	}
}

// Run registers the watches, then runs scan in the background while buffering
// Create events. When scan finishes it drains the buffer — skipping the paths scan
// already consumed — and then dispatches events live. Registering the watches
// before scan runs closes the gap where a file created mid-scan would be seen by
// neither; buffering until scan completes keeps live events from racing the
// scanner's writes into the handler.
func (w *Watcher) Run(ctx context.Context, scan func(context.Context) (map[string]struct{}, error)) error {
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

	scanDone := make(chan map[string]struct{}, 1)
	go func() {
		consumed, err := scan(ctx)
		if err != nil {
			w.logger.LogAttrs(ctx, slog.LevelError, "watcher: scan failed", slog.String("error", err.Error()))
		}
		scanDone <- consumed
	}()

	var buf []string
	scanning := true
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-watcher.Events:
			if event.Op.Has(fsnotify.Create) {
				if scanning {
					buf = append(buf, event.Name)
				} else {
					w.dispatch(ctx, event.Name)
				}
			}
		case consumed := <-scanDone:
			w.drain(ctx, buf, consumed)
			buf, scanning = nil, false
		case err := <-watcher.Errors:
			if err != nil {
				return err
			}
		}
	}
}

// drain dispatches the events buffered during the scan, skipping paths the scan
// already folded into an enqueued round (dispatching those would disrupt the
// handler's in-progress round).
func (w *Watcher) drain(ctx context.Context, buf []string, consumed map[string]struct{}) {
	for _, name := range buf {
		if _, seen := consumed[filepath.Clean(name)]; !seen {
			w.dispatch(ctx, name)
		}
	}
}

func (w *Watcher) dispatch(ctx context.Context, name string) {
	dir := filepath.Dir(name)
	handler, ok := w.handlers[dir]
	if !ok {
		w.logger.LogAttrs(ctx, slog.LevelWarn, "watcher: no server found", slog.String("file", name))
		return
	}

	handler.OnFileCreate(name)
}
