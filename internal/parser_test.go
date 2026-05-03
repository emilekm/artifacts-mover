package internal_test

import (
	"testing"

	"github.com/emilekm/artifacts-mover/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser_ValidSummary(t *testing.T) {
	summary, err := internal.ParseSummary("testdata/valid_summary.json")
	require.NoError(t, err)

	assert.Equal(t, "muttrah_city_2", summary.MapName)
	assert.Equal(t, "gpm_cq", summary.MapMode)
	assert.Equal(t, 64, summary.MapLayer)
	assert.Equal(t, "MEC", summary.Team1Name)
	assert.Equal(t, "USMC", summary.Team2Name)
	assert.Equal(t, 245, summary.Team1Tickets)
	assert.Equal(t, 0, summary.Team2Tickets)
	assert.Equal(t, int64(1700000000), summary.StartTime)
	assert.Equal(t, int64(1700003600), summary.EndTime)
	require.Len(t, summary.Players, 2)
	assert.Equal(t, "Alice", summary.Players[0].Name)
	assert.Equal(t, 150, summary.Players[0].Score)
}

func TestParser_MissingOptionalFields(t *testing.T) {
	summary, err := internal.ParseSummary("testdata/missing_fields_summary.json")
	require.NoError(t, err)

	assert.Equal(t, "muttrah_city_2", summary.MapName)
	assert.Empty(t, summary.Players)
	assert.Zero(t, summary.Team1Tickets)
}

func TestParser_MalformedFile(t *testing.T) {
	_, err := internal.ParseSummary("testdata/malformed_summary.json")
	assert.Error(t, err)
}
