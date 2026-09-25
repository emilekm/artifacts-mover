package notify

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/emilekm/artifacts-mover/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	realJSONPath      = "testdata/tracker_2026_08_13_05_56_48_fallujah_west_gpm_insurgency_16.json"
	realPRDemoPath    = "testdata/tracker_2026_08_13_05_56_48_fallujah_west_gpm_insurgency_16.PRdemo"
	corruptPRDemoPath = "testdata/corrupt_2026_08_13_05_56_48_fallujah_west_gpm_insurgency_16.PRdemo"
	malformedJSONPath = "testdata/malformed_summary.json"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newRound(artifacts ...types.Artifact) types.Round {
	r := make(types.Round, len(artifacts))
	for _, a := range artifacts {
		r[a.Type] = a
	}
	return r
}

func buildSummary(t *testing.T, round types.Round) *Summary {
	t.Helper()
	return BuildSummary(context.Background(), discardLogger(), round)
}

// TestBuildSummary_JSONAndPRDemo covers the common case: both a JSON summary
// and a readable prdemo are present. The prdemo's ticket counts must win over
// the JSON summary's, everything else should agree between the two sources.
func TestBuildSummary_JSONAndPRDemo(t *testing.T) {
	round := newRound(
		types.NewArtifact(realJSONPath, types.ArtifactTypeSummary),
		types.NewArtifact(realPRDemoPath, types.ArtifactTypePRDemo),
	)

	s := buildSummary(t, round)

	assert.Equal(t, "fallujah_west", s.MapName)
	assert.Equal(t, "gpm_insurgency", s.MapMode)
	assert.Equal(t, 16, s.MapLayer)
	assert.Equal(t, "MEInsurgent", s.Team1Name)
	assert.Equal(t, "US", s.Team2Name)
	// This fixture only carries TicketsTeam2Type updates, so only
	// Team2Tickets gets overridden by the prdemo (to 432); Team1Tickets has
	// no prdemo data and falls back to the JSON summary's value (432).
	assert.Equal(t, 432, s.Team1Tickets, "JSON value; prdemo has no TicketsTeam1Type data")
	assert.Equal(t, 432, s.Team2Tickets, "prdemo value, overriding JSON's 0")
	require.NotNil(t, s.StartTime)
	assert.EqualValues(t, 1786600608, *s.StartTime)
	// EndTime came from the JSON summary and must not be recomputed from
	// prdemo ticks, since it was already known.
	require.NotNil(t, s.EndTime)
	assert.EqualValues(t, 1786607858, *s.EndTime, "from JSON, not recomputed")
	assert.Len(t, s.Players, 17)
	assert.NotNil(t, s.PRDemo)
	assert.Equal(t, "tracker_2026_08_13_05_56_48_fallujah_west_gpm_insurgency_16.PRdemo", s.PRDemoName)
}

// TestBuildSummary_JSONOnly covers a round with no prdemo at all: every field
// must come straight from the JSON summary, unmodified.
func TestBuildSummary_JSONOnly(t *testing.T) {
	round := newRound(types.NewArtifact(realJSONPath, types.ArtifactTypeSummary))

	s := buildSummary(t, round)

	assert.Equal(t, "fallujah_west", s.MapName)
	assert.Equal(t, "gpm_insurgency", s.MapMode)
	assert.Equal(t, 16, s.MapLayer)
	assert.Equal(t, 432, s.Team1Tickets, "unmodified JSON value")
	assert.Equal(t, 0, s.Team2Tickets, "unmodified JSON value")
	require.NotNil(t, s.StartTime)
	assert.EqualValues(t, 1786600608, *s.StartTime)
	require.NotNil(t, s.EndTime)
	assert.EqualValues(t, 1786607858, *s.EndTime)
	assert.Len(t, s.Players, 17)
	assert.Nil(t, s.PRDemo, "no prdemo artifact")
	assert.Empty(t, s.PRDemoName)
}

// TestBuildSummary_PRDemoOnly covers the case documented on BuildSummary
// itself: no JSON summary, only a readable prdemo. Every field must be
// reconstructed purely from the prdemo's own content, including a computed
// EndTime.
func TestBuildSummary_PRDemoOnly(t *testing.T) {
	round := newRound(types.NewArtifact(realPRDemoPath, types.ArtifactTypePRDemo))

	s := buildSummary(t, round)

	assert.Equal(t, "fallujah_west", s.MapName)
	assert.Equal(t, "gpm_insurgency", s.MapMode)
	assert.Equal(t, 16, s.MapLayer)
	assert.Equal(t, "MEInsurgent", s.Team1Name)
	assert.Equal(t, "US", s.Team2Name)
	// This fixture only carries TicketsTeam2Type updates; Team1Tickets has
	// no source at all here and stays at its zero value.
	assert.Equal(t, 0, s.Team1Tickets)
	assert.Equal(t, 432, s.Team2Tickets)
	require.NotNil(t, s.StartTime)
	assert.EqualValues(t, 1786600608, *s.StartTime)
	// No JSON EndTime was available, so it must be derived by summing tick
	// deltas from StartTime.
	require.NotNil(t, s.EndTime)
	assert.EqualValues(t, 1786607857, *s.EndTime, "computed from ticks")
	assert.Empty(t, s.Players, "no JSON summary")
	assert.NotNil(t, s.PRDemo)
}

// TestBuildSummary_PRDemoCorruptContent covers a prdemo file that exists but
// whose content can't be decoded (e.g. truncated/corrupted): BuildSummary
// must fall back to whatever the filename itself reveals, without panicking.
func TestBuildSummary_PRDemoCorruptContent(t *testing.T) {
	round := newRound(types.NewArtifact(corruptPRDemoPath, types.ArtifactTypePRDemo))

	s := buildSummary(t, round)

	assert.Equal(t, "fallujah_west", s.MapName, "from filename")
	assert.Equal(t, "gpm_insurgency", s.MapMode, "from filename")
	assert.Equal(t, 16, s.MapLayer, "from filename")
	assert.Empty(t, s.Team1Name, "unreadable content")
	assert.Empty(t, s.Team2Name, "unreadable content")
	assert.Equal(t, 0, s.Team1Tickets, "unreadable content")
	assert.Equal(t, 0, s.Team2Tickets, "unreadable content")
	wantStart := types.NewArtifact(corruptPRDemoPath, types.ArtifactTypePRDemo).Timestamp.Unix()
	require.NotNil(t, s.StartTime)
	assert.EqualValues(t, wantStart, *s.StartTime, "from filename timestamp")
	assert.Nil(t, s.EndTime, "content never read")
	// The raw bytes are still exposed even though they can't be decoded.
	assert.NotNil(t, s.PRDemo, "raw file was read")
}

// TestBuildSummary_PRDemoMissingFile covers a round whose prdemo artifact
// points at a path that doesn't exist on disk: content-derived fields must
// stay empty, but the filename-derived fallback must still run since it
// never touches the file.
func TestBuildSummary_PRDemoMissingFile(t *testing.T) {
	const path = "testdata/does_not_exist_2026_08_13_05_56_48_fallujah_west_gpm_insurgency_16.PRdemo"
	round := newRound(types.NewArtifact(path, types.ArtifactTypePRDemo))

	s := buildSummary(t, round)

	assert.Equal(t, "fallujah_west", s.MapName, "from filename")
	assert.Equal(t, "gpm_insurgency", s.MapMode, "from filename")
	assert.Equal(t, 16, s.MapLayer, "from filename")
	wantStart := types.NewArtifact(path, types.ArtifactTypePRDemo).Timestamp.Unix()
	require.NotNil(t, s.StartTime)
	assert.EqualValues(t, wantStart, *s.StartTime, "from filename timestamp")
	assert.Nil(t, s.PRDemo, "file was never read")
	assert.Empty(t, s.PRDemoName, "file was never read")
}

// TestBuildSummary_BF2DemoOnly covers the last-resort source: only a
// bf2demo artifact is present, so only StartTime can be recovered.
func TestBuildSummary_BF2DemoOnly(t *testing.T) {
	const path = "does/not/matter/auto_2026_08_13_06_00_48.bf2demo"
	round := newRound(types.NewArtifact(path, types.ArtifactTypeBF2Demo))

	s := buildSummary(t, round)

	wantStart := types.NewArtifact(path, types.ArtifactTypeBF2Demo).Timestamp.Unix()
	require.NotNil(t, s.StartTime)
	assert.EqualValues(t, wantStart, *s.StartTime)
	assert.Empty(t, s.MapName)
	assert.Empty(t, s.MapMode)
	assert.Zero(t, s.MapLayer)
	assert.Nil(t, s.EndTime)
	assert.Nil(t, s.PRDemo)
}

// TestBuildSummary_EmptyRound covers a round with no artifacts at all:
// BuildSummary must return a zero-value Summary instead of erroring or
// panicking.
func TestBuildSummary_EmptyRound(t *testing.T) {
	s := buildSummary(t, newRound())

	assert.Empty(t, s.MapName)
	assert.Zero(t, s.Team1Tickets)
	assert.Nil(t, s.StartTime)
	assert.Nil(t, s.EndTime)
	assert.Nil(t, s.PRDemo)
	assert.Empty(t, s.PRDemoName)
}

// TestBuildSummary_JSONMalformed covers a Summary artifact pointing at a
// file that isn't valid JSON: it must be ignored rather than panicking or
// propagating a decode error.
func TestBuildSummary_JSONMalformed(t *testing.T) {
	round := newRound(types.NewArtifact(malformedJSONPath, types.ArtifactTypeSummary))

	s := buildSummary(t, round)

	assert.Empty(t, s.MapName)
	assert.Zero(t, s.Team1Tickets)
	assert.Nil(t, s.StartTime)
}
