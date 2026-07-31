package internal

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/emilekm/artifacts-mover/internal/config"
	applog "github.com/emilekm/artifacts-mover/internal/log"
)

type RoundProcessor struct {
	logger          *slog.Logger
	serverID        string
	uploader        Uploader
	store           StateStore
	notifier        Notifier
	summaries       *SummaryBuilder
	artifactsConfig config.ArtifactsConfig
}

func NewRoundProcessor(
	logger *slog.Logger,
	serverID string,
	uploader Uploader,
	store StateStore,
	notifier Notifier,
	summaries *SummaryBuilder,
	artifactsConfig config.ArtifactsConfig,
) *RoundProcessor {
	return &RoundProcessor{
		logger:          logger,
		serverID:        serverID,
		uploader:        uploader,
		store:           store,
		notifier:        notifier,
		summaries:       summaries,
		artifactsConfig: artifactsConfig,
	}
}

func (p *RoundProcessor) Process(ctx context.Context, round Round) {
	roundKey := time.Now().UTC().Format("20060102-150405.000")

	remoteRefs := make(map[config.ArtifactType]RemoteRef)
	var prDemoPath, summaryPath string

	for typ, artifact := range round {
		artifact.UploadPath = p.artifactsConfig[typ].UploadPath

		if err := p.uploader.Upload(ctx, artifact); err != nil {
			p.logger.LogAttrs(
				ctx, slog.LevelError,
				"round_processor: failed to upload artifact",
				applog.ServerID(p.serverID),
				applog.RoundKey(roundKey),
				applog.ArtifactType(typ),
				applog.Error(err),
			)
			return
		}

		filename := filepath.Base(artifact.Path)
		ref := p.summaries.RemoteRef(typ.String(), filename)

		if err := p.store.RecordUpload(p.serverID, roundKey, UploadedArtifact{
			Filename:  filename,
			Type:      typ,
			RemoteRef: ref,
		}); err != nil {
			p.logger.LogAttrs(
				ctx, slog.LevelError,
				"round_processor: failed to record upload",
				applog.ServerID(p.serverID),
				applog.RoundKey(roundKey),
				applog.ArtifactType(typ),
				applog.Error(err),
			)
			return
		}

		if ref != "" {
			remoteRefs[typ] = ref
		}

		switch typ {
		case config.ArtifactTypePRDemo:
			prDemoPath = artifact.Path
		case config.ArtifactTypeSummary:
			summaryPath = artifact.Path
		}
	}

	summary, err := p.summaries.Build(ctx, summaryPath, prDemoPath, remoteRefs)
	if err != nil {
		p.logger.LogAttrs(
			ctx, slog.LevelError,
			"round_processor: failed to build round summary",
			applog.ServerID(p.serverID),
			applog.RoundKey(roundKey),
			applog.Error(err),
		)
		return
	}

	if err := p.notifier.Send(ctx, summary); err != nil {
		p.logger.LogAttrs(
			ctx, slog.LevelError,
			"round_processor: failed to send notification",
			applog.ServerID(p.serverID),
			applog.RoundKey(roundKey),
			applog.Error(err),
		)
		return
	}

	if err := p.store.RecordNotified(p.serverID, roundKey); err != nil {
		p.logger.LogAttrs(
			ctx, slog.LevelError,
			"round_processor: failed to record notified",
			applog.ServerID(p.serverID),
			applog.RoundKey(roundKey),
			applog.Error(err),
		)
		return
	}

	p.cleanupArtifacts(ctx, round)
}

func (p *RoundProcessor) cleanupArtifacts(ctx context.Context, round Round) {
	for _, artifact := range round {
		if err := os.Remove(artifact.Path); err != nil {
			p.logger.LogAttrs(
				ctx, slog.LevelError,
				"round_processor: failed to remove artifact",
				applog.Path(artifact.Path),
				applog.Error(err),
			)
		}
	}
}
