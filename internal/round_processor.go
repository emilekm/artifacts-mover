package internal

import (
	"context"
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
	}

	if err := p.notifier.Send(ctx, round); err != nil {
		return err
	}

	return p.store.RecordNotified(p.serverID, roundKey)
}
