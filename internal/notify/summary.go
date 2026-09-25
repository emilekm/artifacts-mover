package notify

import (
	"context"
	"io"
	"log/slog"

	"github.com/emilekm/artifacts-mover/internal/types"
)

type Player struct {
	Name  string
	Score int
}

// JSONSummary is the shape written by the game server's tracker script to the
// on-disk JSON summary file.
type JSONSummary struct {
	MapName      string   `json:"MapName"`
	MapMode      string   `json:"MapMode"`
	MapLayer     int      `json:"MapLayer"`
	Team1Name    string   `json:"Team1Name"`
	Team2Name    string   `json:"Team2Name"`
	Team1Tickets int      `json:"Team1Tickets"`
	Team2Tickets int      `json:"Team2Tickets"`
	StartTime    *int64   `json:"StartTime,omitempty"`
	EndTime      *int64   `json:"EndTime,omitempty"`
	Players      []Player `json:"Players"`
}

type Ref struct {
	Enabled bool
	URL     string
}

type RemoteRefs struct {
	BF2Demo       Ref
	PRDemo        Ref
	TrackerViewer Ref
}

// Summary is the reconstructed round summary used for rendering. A field is
// nil when no source was able to determine it.
type Summary struct {
	JSONSummary

	PRDemoName string
	PRDemo     io.Reader
	Image      io.Reader
	RemoteRefs RemoteRefs
}

// setIfZero assigns v to *dst if it isn't already set, so a source never
// overrides a value an earlier, more trusted source already provided.
func setIfZero[T comparable](dst *T, v T) {
	var zero T
	if dst == nil || *dst == zero {
		*dst = v
	}
}

// BuildSummary reconstructs a round's summary from whichever sources are
// available. Sources run in order of decreasing trust, each filling in
// whatever gaps are left; a round with no JSON summary and no readable prdemo
// content still yields a (mostly empty) Summary rather than an error.
func BuildSummary(ctx context.Context, logger *slog.Logger, round types.Round) *Summary {
	s := &Summary{}

	sourceJSONFile(ctx, logger, round, s)
	sourcePRDemoContent(ctx, logger, round, s)
	sourcePRDemoFilename(round, s)
	sourceBF2DemoFilename(round, s)

	return s
}
