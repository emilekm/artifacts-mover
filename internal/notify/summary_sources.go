package notify

import (
	"context"
	"encoding/json"
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

	var js JSONSummary
	if err := json.NewDecoder(file).Decode(&js); err != nil {
		logger.LogAttrs(
			ctx, slog.LevelWarn,
			"summary: failed to decode json summary",
			applog.Path(artifact.Path),
			applog.Error(err),
		)
		return
	}

	setIfNil(&s.MapName, js.MapName)
	setIfNil(&s.MapMode, js.MapMode)
	setIfNil(&s.MapLayer, js.MapLayer)
	setIfNil(&s.Team1Name, js.Team1Name)
	setIfNil(&s.Team2Name, js.Team2Name)
	setIfNil(&s.Team1Tickets, js.Team1Tickets)
	setIfNil(&s.Team2Tickets, js.Team2Tickets)
	setIfNil(&s.StartTime, js.StartTime)
	setIfNil(&s.EndTime, js.EndTime)
	if len(js.Players) > 0 {
		s.Players = js.Players
	}
}

// sourcePRDemoContent walks the prdemo file once, reconstructing whatever the
// JSON summary source left unset from the ServerDetails message, and always
// overriding the ticket counts with the ones recorded immediately before
// RoundEnd (the JSON summary's tickets are unreliable). EndTime is derived by
// summing tick deltas from StartTime, but only if it's still unknown.
func sourcePRDemoContent(ctx context.Context, logger *slog.Logger, s *Summary) {
	demo, err := prdemo.NewDemoReaderFromFile(s.PRDemoPath)
	if err != nil {
		logger.LogAttrs(
			ctx, slog.LevelWarn,
			"summary: failed to open prdemo",
			applog.Path(s.PRDemoPath),
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
				applog.Path(s.PRDemoPath),
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
			setIfNil(&s.MapName, d.Map.Name)
			setIfNil(&s.MapMode, d.Map.Gamemode)
			setIfNil(&s.MapLayer, int(d.Map.Layer))
			setIfNil(&s.Team1Name, d.BluforTeam)
			setIfNil(&s.Team2Name, d.OpforTeam)
			startTime = int64(d.StartTime)
			setIfNil(&s.StartTime, startTime)
		case prdemo.TicketsTeam1Type, prdemo.TicketsTeam2Type:
			var t prdemo.Tickets
			if err := msg.Decode(&t); err != nil {
				continue
			}
			if t.Tickets < 0 {
				t.Tickets = 0
			}
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
		s.Team1Tickets = &team1
	}
	if haveTeam2 {
		s.Team2Tickets = &team2
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
	artifact := round[types.ArtifactTypePRDemo]

	if !artifact.Timestamp.IsZero() {
		startTime := artifact.Timestamp.Unix()
		setIfNil(&s.StartTime, startTime)
	}

	m := prDemoFilenameRe.FindStringSubmatch(filepath.Base(artifact.Path))
	if m == nil {
		return
	}

	setIfNil(&s.MapName, m[1])
	setIfNil(&s.MapMode, m[2])
	if layer, err := strconv.Atoi(m[3]); err == nil {
		setIfNil(&s.MapLayer, layer)
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
	setIfNil(&s.StartTime, startTime)
}
