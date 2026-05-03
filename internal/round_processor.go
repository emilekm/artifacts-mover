package internal

import (
	"context"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/emilekm/artifacts-mover/internal/config"
)

type artifactUploader interface {
	Upload(ctx context.Context, artifact Artifact) (RemoteRef, error)
}

type RoundProcessor struct {
	serverID        string
	uploader        artifactUploader
	store           StateStore
	notifier        Notifier
	artifactsConfig config.ArtifactsConfig
}

func NewRoundProcessor(
	serverID string,
	uploader artifactUploader,
	store StateStore,
	notifier Notifier,
	artifactsConfig config.ArtifactsConfig,
) *RoundProcessor {
	return &RoundProcessor{
		serverID:        serverID,
		uploader:        uploader,
		store:           store,
		notifier:        notifier,
		artifactsConfig: artifactsConfig,
	}
}

func (p *RoundProcessor) Process(ctx context.Context, round Round) error {
	roundKey := time.Now().UTC().Format("20060102-150405.000")

	remoteRefs := make(map[config.ArtifactType]RemoteRef)
	var prDemoPath, summaryPath string

	for typ, artifact := range round {
		artifact.UploadPath = p.artifactsConfig[typ].UploadPath

		ref, err := p.uploader.Upload(ctx, artifact)
		if err != nil {
			return err
		}

		if err := p.store.RecordUpload(p.serverID, roundKey, UploadedArtifact{
			Filename:  filepath.Base(artifact.Path),
			Type:      typ,
			RemoteRef: ref,
		}); err != nil {
			return err
		}

		remoteRefs[typ] = ref

		switch typ {
		case config.ArtifactTypePRDemo:
			prDemoPath = artifact.Path
		case config.ArtifactTypeSummary:
			summaryPath = artifact.Path
		}
	}

	summary, err := ParseSummary(summaryPath)
	if err != nil {
		return err
	}

	if prDemoPath != "" {
		t1, t2, err := ExtractTickets(prDemoPath)
		if err != nil {
			slog.Warn("failed to extract tickets from prdemo", "path", prDemoPath, "err", err)
		} else {
			summary.Team1Tickets = int(t1)
			summary.Team2Tickets = int(t2)
		}
	}

	summary.PRDemoPath = prDemoPath
	summary.RemoteRefs = remoteRefs

	if err := p.notifier.Send(ctx, summary); err != nil {
		return err
	}

	return p.store.RecordNotified(p.serverID, roundKey)
}
