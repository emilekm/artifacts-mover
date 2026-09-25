package notify

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	applog "github.com/emilekm/artifacts-mover/internal/log"
	"github.com/emilekm/artifacts-mover/internal/types"
	"github.com/emilekm/go-prbf2/prdemo"
)

// secondsPerTick is the resolution of the TicksType message payload, per
// onTrackerTick() in the tracker script: each tick is the elapsed wall time
// since the previous tick, in 0.04s units, capped at a uint8 (255).
const secondsPerTick = 0.04

var prDemoFilenameRe = regexp.MustCompile(`\d{4}_\d{2}_\d{2}_\d{2}_\d{2}_\d{2}_(.+)_(gpm_[a-z]+)_(\d+)\.[A-Za-z]+$`)

// sourceJSONFile fills the summary from the on-disk JSON summary file, when
// present. Its ticket counts are unreliable and are always overridden by
// sourcePRDemoContent when the prdemo can be read.
func sourceJSONFile(ctx context.Context, logger *slog.Logger, round types.Round, s *Summary) {
	artifact, ok := round[types.ArtifactTypeSummary]
	if !ok {
		return
	}

	file, err := os.Open(artifact.Path)
	if err != nil {
		logger.LogAttrs(
			ctx, slog.LevelWarn,
			"summary: failed to open json summary",
			applog.Path(artifact.Path),
			applog.Error(err),
		)
		return
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(&s.JSONSummary); err != nil {
		logger.LogAttrs(
			ctx, slog.LevelWarn,
			"summary: failed to decode json summary",
			applog.Path(artifact.Path),
			applog.Error(err),
		)
		return
	}
}

// sourcePRDemoContent walks the prdemo file once, reconstructing whatever the
// JSON summary source left unset from the ServerDetails message, and always
// overriding the ticket counts with the ones recorded immediately before
// RoundEnd (the JSON summary's tickets are unreliable). EndTime is derived by
// summing tick deltas from StartTime, but only if it's still unknown.
func sourcePRDemoContent(ctx context.Context, logger *slog.Logger, round types.Round, s *Summary) {
	artifact, ok := round[types.ArtifactTypePRDemo]
	if !ok {
		return
	}

	demoBuffer, err := os.ReadFile(artifact.Path)
	if err != nil {
		logger.LogAttrs(
			ctx, slog.LevelWarn,
			"summary: failed to read prdemo file",
			applog.Path(artifact.Path),
			applog.Error(err),
		)
		return
	}

	s.PRDemo = bytes.NewReader(demoBuffer)
	s.PRDemoName = filepath.Base(artifact.Path)

	demo, err := openDemo(bytes.NewReader(demoBuffer))
	if err != nil {
		logger.LogAttrs(
			ctx, slog.LevelWarn,
			"summary: failed to open prdemo",
			applog.Path(artifact.Path),
			applog.Error(err),
		)
		return
	}

	computeEndTime := s.EndTime == nil
	var startTime int64
	var elapsed float64
	var team1, team2 int
	var haveTeam1, haveTeam2 bool

loop:
	for demo.Next() {
		msg, err := demo.GetMessage()
		if err != nil {
			logger.LogAttrs(
				ctx, slog.LevelWarn,
				"summary: failed to read prdemo message",
				applog.Path(artifact.Path),
				applog.Error(err),
			)
			break
		}

		switch msg.Type {
		case prdemo.ServerDetailsType:
			var d prdemo.ServerDetails
			if err := msg.Decode(&d); err != nil {
				continue
			}
			setIfZero(&s.MapName, d.Map.Name)
			setIfZero(&s.MapMode, d.Map.Gamemode)
			setIfZero(&s.MapLayer, int(d.Map.Layer))
			setIfZero(&s.Team1Name, d.BluforTeam)
			setIfZero(&s.Team2Name, d.OpforTeam)
			startTime = int64(d.StartTime)
			st := startTime
			setIfZero(&s.StartTime, &st)
		case prdemo.TicketsTeam1Type, prdemo.TicketsTeam2Type:
			var t prdemo.Tickets
			if err := msg.Decode(&t); err != nil {
				continue
			}
			if t.Tickets < 0 {
				t.Tickets = 0
			}
			// The tracker script that produces these prdemos swaps which
			// team's tickets it tags as TicketsTeam1Type/TicketsTeam2Type
			// relative to BluforTeam/OpforTeam (Team1Name/Team2Name) above,
			// so the mapping here is intentionally inverted.
			switch msg.Type {
			case prdemo.TicketsTeam1Type:
				team1 = int(t.Tickets)
				haveTeam1 = true
			case prdemo.TicketsTeam2Type:
				team2 = int(t.Tickets)
				haveTeam2 = true
			}
		case prdemo.TicksType:
			if !computeEndTime {
				continue
			}
			var tick uint8
			if err := msg.Decode(&tick); err != nil {
				continue
			}
			elapsed += float64(tick) * secondsPerTick
		case prdemo.RoundEndType:
			break loop
		}
	}

	if haveTeam1 {
		if team1 < 0 {
			team1 = 0
		}
		s.Team1Tickets = team1
	}
	if haveTeam2 {
		if team2 < 0 {
			team2 = 0
		}
		s.Team2Tickets = team2
	}

	if computeEndTime && startTime > 0 {
		endTime := startTime + int64(elapsed)
		s.EndTime = &endTime
	}
}

// sourcePRDemoFilename fills MapName/MapMode/MapLayer from the prdemo's own
// filename, and StartTime from its filename timestamp - both readable even
// when the prdemo's content is corrupt or unreadable.
func sourcePRDemoFilename(round types.Round, s *Summary) {
	artifact, ok := round[types.ArtifactTypePRDemo]
	if !ok {
		return
	}

	if !artifact.Timestamp.IsZero() {
		startTime := artifact.Timestamp.Unix()
		setIfZero(&s.StartTime, &startTime)
	}

	m := prDemoFilenameRe.FindStringSubmatch(filepath.Base(artifact.Path))
	if m == nil {
		return
	}

	setIfZero(&s.MapName, m[1])
	setIfZero(&s.MapMode, m[2])
	if layer, err := strconv.Atoi(m[3]); err == nil {
		setIfZero(&s.MapLayer, layer)
	}
}

// sourceBF2DemoFilename is the last resort for StartTime, used only when the
// prdemo's own filename didn't carry a parseable timestamp.
func sourceBF2DemoFilename(round types.Round, s *Summary) {
	artifact, ok := round[types.ArtifactTypeBF2Demo]
	if !ok || artifact.Timestamp.IsZero() {
		return
	}

	startTime := artifact.Timestamp.Unix()
	setIfZero(&s.StartTime, &startTime)
}

func openDemo(reader io.Reader) (prdemo.DemoReader, error) {
	zReader, err := zlib.NewReader(reader)
	if err != nil {
		return nil, err
	}
	defer zReader.Close()

	buf, err := io.ReadAll(zReader)
	if err != nil {
		return nil, err
	}

	return prdemo.NewDemoReader(bytes.NewReader(buf))
}
