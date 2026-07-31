package internal

import (
	"context"
	"log/slog"
	"path/filepath"
	"sort"
	"time"

	"github.com/emilekm/artifacts-mover/internal/config"
)

type Recovery struct {
	logger          *slog.Logger
	serverID        string
	store           StateStore
	handler         fileHandler
	summaries       *SummaryBuilder
	notifier        Notifier
	artifactsConfig config.ArtifactsConfig
}

func NewRecovery(
	logger *slog.Logger,
	serverID string,
	store StateStore,
	handler fileHandler,
	summaries *SummaryBuilder,
	notifier Notifier,
	artifactsConfig config.ArtifactsConfig,
) *Recovery {
	return &Recovery{
		logger:          logger,
		serverID:        serverID,
		store:           store,
		handler:         handler,
		summaries:       summaries,
		notifier:        notifier,
		artifactsConfig: artifactsConfig,
	}
}

func (r *Recovery) ReplayUnnotified(ctx context.Context, since time.Time) error {
	records, err := r.store.QueryUnnotified(r.serverID, since)
	if err != nil {
		return err
	}

	for _, record := range records {
		if err := r.retryNotification(ctx, record); err != nil {
			slog.Warn("failed to retry notification", "roundKey", record.RoundKey, "err", err)
		}
	}

	return nil
}

func (r *Recovery) retryNotification(ctx context.Context, record RoundRecord) error {
	remoteRefs := make(map[config.ArtifactType]RemoteRef)
	var summaryFilename, prDemoFilename string

	for _, a := range record.Artifacts {
		if a.RemoteRef != "" {
			remoteRefs[a.Type] = a.RemoteRef
		}
		switch a.Type {
		case config.ArtifactTypeSummary:
			summaryFilename = a.Filename
		case config.ArtifactTypePRDemo:
			prDemoFilename = a.Filename
		}
	}

	summaryPath := ""
	if summaryFilename != "" {
		summaryPath = filepath.Join(r.artifactsConfig[config.ArtifactTypeSummary].Location, summaryFilename)
	}

	prDemoPath := ""
	if prDemoFilename != "" {
		prDemoPath = filepath.Join(r.artifactsConfig[config.ArtifactTypePRDemo].Location, prDemoFilename)
	}

	summary, err := r.summaries.Build(ctx, summaryPath, prDemoPath, remoteRefs)
	if err != nil {
		return err
	}

	if err := r.notifier.Send(ctx, summary); err != nil {
		return err
	}

	return r.store.RecordNotified(r.serverID, record.RoundKey)
}

func (r *Recovery) UploadOldFiles(ctx context.Context) error {
	allFiles := make(map[config.ArtifactType][]string)

	maxLen := 0
	for typ, loc := range r.artifactsConfig {
		files, err := filepath.Glob(filepath.Join(loc.Location, "*"))
		if err != nil {
			return err
		}
		sort.Strings(files)
		allFiles[typ] = files
		if len(files) > maxLen {
			maxLen = len(files)
		}
	}

	for i := range maxLen {
		if files := allFiles[config.ArtifactTypeBF2Demo]; i < len(files) {
			r.handler.OnFileCreate(files[i])
		}
		for typ, files := range allFiles {
			if typ == config.ArtifactTypeBF2Demo {
				continue
			}
			if i < len(files) {
				r.handler.OnFileCreate(files[i])
			}
		}
	}

	return nil
}
