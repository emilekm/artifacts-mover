package internal

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/emilekm/artifacts-mover/internal/config"
)

type RoundProcessor struct {
	serverID        string
	uploader        Uploader
	store           StateStore
	notifier        Notifier
	artifactsConfig config.ArtifactsConfig
	discordURLs     map[string]string
}

func NewRoundProcessor(
	serverID string,
	uploader Uploader,
	store StateStore,
	notifier Notifier,
	artifactsConfig config.ArtifactsConfig,
	discordURLs map[string]string,
) *RoundProcessor {
	return &RoundProcessor{
		serverID:        serverID,
		uploader:        uploader,
		store:           store,
		notifier:        notifier,
		artifactsConfig: artifactsConfig,
		discordURLs:     discordURLs,
	}
}

func (p *RoundProcessor) Process(ctx context.Context, round Round) error {
	roundKey := time.Now().UTC().Format("20060102-150405.000")

	remoteRefs := make(map[config.ArtifactType]RemoteRef)
	var prDemoPath, summaryPath string

	for typ, artifact := range round {
		artifact.UploadPath = p.artifactsConfig[typ].UploadPath

		if err := p.uploader.Upload(ctx, artifact); err != nil {
			return err
		}

		filename := filepath.Base(artifact.Path)
		ref := p.remoteRef(typ.String(), filename)

		if err := p.store.RecordUpload(p.serverID, roundKey, UploadedArtifact{
			Filename:  filename,
			Type:      typ,
			RemoteRef: ref,
		}); err != nil {
			return err
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

	summary, err := p.buildSummary(summaryPath, prDemoPath, remoteRefs)
	if err != nil {
		return err
	}

	if err := p.notifier.Send(ctx, summary); err != nil {
		return err
	}

	if err := p.store.RecordNotified(p.serverID, roundKey); err != nil {
		return err
	}

	p.cleanupArtifacts(round)
	return nil
}

// ReplayUnnotified retries Discord notifications for rounds that were fully
// uploaded but never notified (e.g. due to a crash). Only rounds with a
// CreatedAt after `since` are retried.
func (p *RoundProcessor) ReplayUnnotified(ctx context.Context, since time.Time) error {
	records, err := p.store.QueryUnnotified(p.serverID, since)
	if err != nil {
		return err
	}

	for _, record := range records {
		if err := p.retryNotification(ctx, record); err != nil {
			slog.Warn("failed to retry notification", "roundKey", record.RoundKey, "err", err)
		}
	}

	return nil
}

func (p *RoundProcessor) retryNotification(ctx context.Context, record RoundRecord) error {
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
		summaryPath = filepath.Join(p.artifactsConfig[config.ArtifactTypeSummary].Location, summaryFilename)
	}

	prDemoPath := ""
	if prDemoFilename != "" {
		prDemoPath = filepath.Join(p.artifactsConfig[config.ArtifactTypePRDemo].Location, prDemoFilename)
	}

	summary, err := p.buildSummary(summaryPath, prDemoPath, remoteRefs)
	if err != nil {
		return err
	}

	if err := p.notifier.Send(ctx, summary); err != nil {
		return err
	}

	return p.store.RecordNotified(p.serverID, record.RoundKey)
}

func (p *RoundProcessor) buildSummary(summaryPath, prDemoPath string, remoteRefs map[config.ArtifactType]RemoteRef) (*RoundSummary, error) {
	summary, err := ParseSummary(summaryPath)
	if err != nil {
		return nil, err
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
	return summary, nil
}

// UploadOldFiles scans each artifact type's watch directory, sorts files
// alphabetically, and processes them as rounds by index position. This
// recovers complete rounds that arrived while the process was down.
func (p *RoundProcessor) UploadOldFiles(ctx context.Context) error {
	allFiles := make(map[config.ArtifactType][]string)

	for typ, loc := range p.artifactsConfig {
		files, err := filepath.Glob(filepath.Join(loc.Location, "*"))
		if err != nil {
			return err
		}
		sort.Strings(files)
		allFiles[typ] = files
	}

	maxLen := 0
	for _, files := range allFiles {
		if len(files) > maxLen {
			maxLen = len(files)
		}
	}

	for i := range maxLen {
		round := make(Round)
		for typ, files := range allFiles {
			if i < len(files) {
				round[typ] = Artifact{Path: files[i], Type: typ}
			}
		}
		if len(round) == 0 {
			continue
		}
		if err := p.Process(ctx, round); err != nil {
			slog.Error("failed to process old round", "index", i, "err", err)
		}
	}

	return nil
}

func (p *RoundProcessor) remoteRef(typKey, filename string) string {
	if urlTemplate, ok := p.discordURLs[typKey]; ok {
		return fmt.Sprintf(urlTemplate, filename)
	}
	return ""
}

func (p *RoundProcessor) cleanupArtifacts(round Round) {
	for typ, artifact := range round {
		cfg := p.artifactsConfig[typ]
		if cfg.MovePath != nil {
			dst := filepath.Join(*cfg.MovePath, filepath.Base(artifact.Path))
			if err := move(artifact.Path, dst); err != nil {
				slog.Error("failed to move artifact", "src", artifact.Path, "dst", dst, "err", err)
			}
		} else {
			if err := os.Remove(artifact.Path); err != nil {
				slog.Error("failed to remove artifact", "path", artifact.Path, "err", err)
			}
		}
	}
}
