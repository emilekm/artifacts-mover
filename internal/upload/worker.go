package upload

import (
	"context"
	"log/slog"
	"time"

	applog "github.com/emilekm/artifacts-mover/internal/log"
	"github.com/emilekm/artifacts-mover/internal/store"
	"github.com/emilekm/artifacts-mover/internal/types"
)

type Uploader interface {
	Upload(ctx context.Context, a types.Artifact) error
}

type Worker struct {
	logger *slog.Logger
	store  store.Store

	uploaders map[string]Uploader
}

func NewWorker(logger *slog.Logger, stateStore store.Store) *Worker {
	return &Worker{
		logger:    logger,
		store:     stateStore,
		uploaders: make(map[string]Uploader),
	}
}

func (w *Worker) Register(serverID string, uploader Uploader) {
	w.uploaders[serverID] = uploader
}

func (w *Worker) Watch(ctx context.Context) error {
	ticker := time.NewTicker(time.Second)
	for {
		records, err := w.store.PendingUploads()
		if err == nil {
			for _, record := range records {
				w.uploadRecord(ctx, record)
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (w *Worker) uploadRecord(ctx context.Context, record types.Round) {
	uploader, ok := w.uploaders[record.ServerID]
	if !ok {
		w.logger.LogAttrs(
			ctx, slog.LevelError,
			"upload: didnt find handler",
			applog.ServerID(record.ServerID),
		)
		return
	}

	for _, artifact := range record.Artifacts {
		if artifact.Uploaded {
			continue
		}

		err := uploader.Upload(ctx, artifact)
		if err != nil {
			w.logger.LogAttrs(
				ctx, slog.LevelError,
				"upload: failed to upload artifact",
				applog.ServerID(record.ServerID),
				applog.RoundID(record.RoundID),
				applog.Error(err),
			)
			continue
		}

		err = w.store.MarkUploaded(record.ServerID, record.RoundID, artifact.Type)
		if err != nil {
			w.logger.LogAttrs(
				ctx, slog.LevelError,
				"upload: failed to record upload",
				applog.ServerID(record.ServerID),
				applog.RoundID(record.RoundID),
				applog.ArtifactType(artifact.Type),
				applog.Error(err),
			)
		}
	}
}
