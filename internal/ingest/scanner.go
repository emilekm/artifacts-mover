package ingest

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"time"

	"github.com/emilekm/artifacts-mover/internal/config"
	"github.com/emilekm/artifacts-mover/internal/db"
	"github.com/emilekm/artifacts-mover/internal/log"
	applog "github.com/emilekm/artifacts-mover/internal/log"
	"github.com/emilekm/artifacts-mover/internal/types"
	"github.com/emilekm/go-prbf2/prdemo"
	"github.com/riverqueue/river"
)

// Scanner rebuilds the rounds that already exist on disk at startup and enqueues
// them, so a crash mid-round doesn't lose files. The Watcher runs Scan once, then
// takes over live from there.
type Scanner struct {
	logger    *slog.Logger
	db        *db.DB
	river     *river.Client[*sql.Tx]
	handler   fileHandler
	artifacts config.ArtifactsConfig
	serverID  string
}

func NewScanner(
	logger *slog.Logger,
	db *db.DB,
	river *river.Client[*sql.Tx],
	handler fileHandler,
	artifactsConfig config.ArtifactsConfig,
	serverID string,
) *Scanner {
	return &Scanner{
		logger:    logger,
		db:        db,
		river:     river,
		handler:   handler,
		artifacts: artifactsConfig,
		serverID:  serverID,
	}
}

// Scan reconstructs the rounds already on disk, enqueues them, seeds the running
// round into the handler, and returns the set of file paths it consumed so the
// watcher can skip them when it replays events buffered during the scan.
func (s *Scanner) Scan(ctx context.Context) (map[string]struct{}, error) {
	prdemos, err := s.list(types.ArtifactTypePRDemo)
	if err != nil {
		return nil, err
	}
	bf2demos, err := s.list(types.ArtifactTypeBF2Demo)
	if err != nil {
		return nil, err
	}
	summaries, err := s.list(types.ArtifactTypeSummary)
	if err != nil {
		return nil, err
	}

	rounds, running, anomalies := matchRounds(s.serverID, prdemos, bf2demos, summaries, decodeBriefing)

	// matchRounds emits the bf2demo-matched rounds before the unmatched ones, so
	// sort before enqueuing: the notify worker publishes what it finds while the
	// scan is still running, and every round it finds has to be older than the
	// ones still to come.
	sort.Slice(rounds, func(i, j int) bool {
		return rounds[i][types.ArtifactTypePRDemo].Timestamp.Before(rounds[j][types.ArtifactTypePRDemo].Timestamp)
	})

	consumed := make(map[string]struct{})
	for _, round := range rounds {
		if err := ctx.Err(); err != nil {
			return consumed, err
		}

		timestamp := round[types.ArtifactTypePRDemo].Timestamp

		err := s.db.EnqueueRound(ctx, s.river, s.serverID, timestamp, round)
		if err != nil {
			s.logger.LogAttrs(
				ctx, slog.LevelError,
				"handler: failed to enqueue round",
				log.ServerID(s.serverID),
				log.Error(err),
			)
		}
		for _, artifact := range round {
			consumed[filepath.Clean(artifact.Path)] = struct{}{}
		}
	}

	for _, bf := range anomalies {
		s.logger.LogAttrs(
			ctx, slog.LevelWarn,
			"scanner: bf2demo has no matching prdemo",
			applog.ServerID(s.serverID),
			applog.Path(bf.Path),
		)
	}

	if running != nil {
		s.handler.OnFileCreate(running.Path)
		consumed[filepath.Clean(running.Path)] = struct{}{}
	}

	return consumed, nil
}

// list globs the directory for the given artifact type and returns the
// timestamped files sorted ascending by timestamp. Files without a timestamp in
// their name (and directories) are skipped.
func (s *Scanner) list(typ types.ArtifactType) ([]types.Artifact, error) {
	loc, ok := s.artifacts[typ]
	if !ok {
		return nil, nil
	}

	paths, err := filepath.Glob(filepath.Join(loc.Location, "*"))
	if err != nil {
		return nil, fmt.Errorf("scanner: listing %s: %w", typ, err)
	}

	var out []types.Artifact
	for _, path := range paths {
		a := types.NewArtifact(path, typ)
		if a.Timestamp == (time.Time{}) {
			continue
		}
		out = append(out, a)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Timestamp.Before(out[j].Timestamp)
	})
	return out, nil
}

type prdemoEntry struct {
	artifact types.Artifact
	matched  bool
}

// matchRounds reconstructs rounds from the sorted artifact slices. Each bf2demo
// belongs to the nearest preceding prdemo; the {B-1, B} briefing-time window
// (B = the prdemo's decoded briefing seconds) confirms the match and rejects a
// bf2demo whose real prdemo is missing or corrupt. Summaries join a round by
// sharing the prdemo's filename timestamp.
//
// It returns the rounds to enqueue; the latest bf2demo with no matching prdemo as
// the still-running round to seed into the live handler (nil if none); and any
// earlier bf2demos with no matching prdemo as anomalies (missing/corrupt prdemo).
func matchRounds(
	serverID string,
	prdemos, bf2demos, summaries []types.Artifact,
	briefing func(path string) (int, error),
) ([]types.Round, *types.Artifact, []types.Artifact) {
	entries := make([]prdemoEntry, len(prdemos))
	for i, pr := range prdemos {
		entries[i] = prdemoEntry{artifact: pr}
	}

	summaryByTS := make(map[time.Time]types.Artifact, len(summaries))
	for _, sm := range summaries {
		summaryByTS[sm.Timestamp] = sm
	}

	var rounds []types.Round
	var running *types.Artifact
	var anomalies []types.Artifact

	pi := 0
	var cur *prdemoEntry
	for bi := range bf2demos {
		bf := bf2demos[bi]
		for pi < len(entries) && !entries[pi].artifact.Timestamp.After(bf.Timestamp) {
			cur = &entries[pi]
			pi++
		}

		if cur != nil {
			gap := int(bf.Timestamp.Sub(cur.artifact.Timestamp) / time.Second)
			if b, err := briefing(cur.artifact.Path); err == nil && (gap == b || gap == b-1) {
				cur.matched = true
				rounds = append(rounds, buildRound(serverID, cur.artifact, &bf, summaryByTS))
				continue
			}
		}

		// The latest bf2demo without a prdemo is a round still in progress; any
		// earlier one means its prdemo is missing or corrupt.
		if bi == len(bf2demos)-1 {
			running = &bf2demos[bi]
		} else {
			anomalies = append(anomalies, bf)
		}
	}

	for i := range entries {
		if !entries[i].matched {
			rounds = append(rounds, buildRound(serverID, entries[i].artifact, nil, summaryByTS))
		}
	}

	return rounds, running, anomalies
}

func buildRound(
	serverID string,
	prDemo types.Artifact,
	bf2Demo *types.Artifact,
	summaryByTS map[time.Time]types.Artifact,
) types.Round {
	round := make(types.Round)
	round[types.ArtifactTypePRDemo] = prDemo

	if bf2Demo != nil {
		round[types.ArtifactTypeBF2Demo] = *bf2Demo
	}
	if summary, ok := summaryByTS[prDemo.Timestamp]; ok {
		round[types.ArtifactTypeSummary] = summary
	}

	return round
}

func decodeBriefing(path string) (int, error) {
	r, err := prdemo.NewDemoReaderFromFile(path)
	if err != nil {
		return 0, err
	}
	for r.Next() {
		msg, err := r.GetMessage()
		if err != nil {
			return 0, err
		}
		if msg.Type == prdemo.ServerDetailsType {
			var d prdemo.ServerDetails
			if err := msg.Decode(&d); err != nil {
				return 0, err
			}
			return int(d.BriefingTime), nil
		}
	}
	return 0, fmt.Errorf("no ServerDetails message found")
}
