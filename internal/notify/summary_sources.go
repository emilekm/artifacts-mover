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
	"github.com/emilekm/go-prbf2/bf2demo"
	"github.com/emilekm/go-prbf2/prdemo"
)

const secondsPerTick = 0.04

var prDemoFilenameRe = regexp.MustCompile(`\d{4}_\d{2}_\d{2}_\d{2}_\d{2}_\d{2}_(.+)_(gpm_[a-z]+)_(\d+)\.[A-Za-z]+$`)

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

	s.Team1Tickets, s.Team2Tickets = s.Team2Tickets, s.Team1Tickets
}

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
			setIfZero(&s.Team1Name, d.Team1Name)
			setIfZero(&s.Team2Name, d.Team2Name)
			startTime = int64(d.StartTime)
			st := startTime
			setIfZero(&s.StartTime, &st)
			team1 = int(d.Tickets1)
			team2 = int(d.Tickets2)
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
			case prdemo.TicketsTeam2Type:
				team2 = int(t.Tickets)
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

	s.Team1Tickets = team1
	s.Team2Tickets = team2

	if computeEndTime && startTime > 0 {
		endTime := startTime + int64(elapsed)
		s.EndTime = &endTime
	}
}

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

func sourceBF2DemoFilename(ctx context.Context, logger *slog.Logger, round types.Round, s *Summary) {
	artifact, ok := round[types.ArtifactTypeBF2Demo]
	if !ok || artifact.Timestamp.IsZero() {
		return
	}

	startTime := artifact.Timestamp.Unix()
	setIfZero(&s.StartTime, &startTime)

	if s.MapName != "" {
		return
	}

	f, err := bf2demo.Open(artifact.Path)
	if err != nil {
		logger.LogAttrs(
			ctx, slog.LevelWarn,
			"summary: failed to open bf2demo",
			applog.Path(artifact.Path),
			applog.Error(err),
		)
		return
	}
	defer f.Close()

	meta, err := bf2demo.DecodeMetadata(f)
	if err != nil {
		logger.LogAttrs(
			ctx, slog.LevelWarn,
			"summary: failed to decode bf2demo metadata",
			applog.Path(artifact.Path),
			applog.Error(err),
		)
		return
	}

	s.MapName = meta.MapName
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
