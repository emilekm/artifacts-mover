package internal

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/emilekm/artifacts-mover/internal/config"
	applog "github.com/emilekm/artifacts-mover/internal/log"
)

type SummaryBuilder struct {
	logger      *slog.Logger
	discordURLs map[string]string
}

func NewSummaryBuilder(logger *slog.Logger, discordURLs map[string]string) *SummaryBuilder {
	return &SummaryBuilder{logger: logger, discordURLs: discordURLs}
}

func (b *SummaryBuilder) Build(
	ctx context.Context,
	summaryPath, prDemoPath string,
	remoteRefs map[config.ArtifactType]RemoteRef,
) (*RoundSummary, error) {
	summary, err := ParseSummary(summaryPath)
	if err != nil {
		return nil, err
	}

	if prDemoPath != "" {
		t1, t2, err := ExtractTickets(prDemoPath)
		if err != nil {
			b.logger.LogAttrs(
				ctx, slog.LevelWarn,
				"summary_builder: failed to extract tickets from prdemo",
				applog.Path(prDemoPath),
				applog.Error(err),
			)
		} else {
			summary.Team1Tickets = int(t1)
			summary.Team2Tickets = int(t2)
		}
	}

	summary.PRDemoPath = prDemoPath
	summary.RemoteRefs = remoteRefs
	return summary, nil
}

func (b *SummaryBuilder) RemoteRef(typKey, filename string) string {
	if urlTemplate, ok := b.discordURLs[typKey]; ok {
		return fmt.Sprintf(urlTemplate, filename)
	}
	return ""
}
