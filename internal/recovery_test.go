package internal

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/emilekm/artifacts-mover/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRecovery(t *testing.T, store StateStore, notifier Notifier, summaries *SummaryBuilder, cfg config.ArtifactsConfig) *Recovery {
	t.Helper()
	proc := NewRoundProcessor(testLogger(t), "server1", &mockArtifactUploader{}, store, notifier, summaries, cfg)
	handler, err := NewHandler(testLogger(t), proc, cfg, 0)
	require.NoError(t, err)
	return NewRecovery(testLogger(t), "server1", store, handler, summaries, notifier, cfg)
}

func TestRecovery_UploadOldFiles_PairsFilesByIndex(t *testing.T) {
	summaryDir := t.TempDir()
	prDemoDir := t.TempDir()

	summaryContent, err := os.ReadFile("testdata/valid_summary.json")
	require.NoError(t, err)

	for _, name := range []string{"20250501-round1.json", "20250502-round2.json"} {
		require.NoError(t, os.WriteFile(filepath.Join(summaryDir, name), summaryContent, 0644))
	}
	for _, name := range []string{"20250501-round1.prdemo", "20250502-round2.prdemo"} {
		require.NoError(t, os.WriteFile(filepath.Join(prDemoDir, name), []byte("prdemo"), 0644))
	}

	store := newTestStore(t)
	notifier := &mockRoundNotifier{}

	cfg := config.ArtifactsConfig{
		config.ArtifactTypeSummary: config.Location{Location: summaryDir, UploadPath: "summaries"},
		config.ArtifactTypePRDemo:  config.Location{Location: prDemoDir, UploadPath: "prdemos"},
	}

	recovery := newTestRecovery(t, store, notifier, NewSummaryBuilder(testLogger(t), nil), cfg)

	require.NoError(t, recovery.UploadOldFiles(context.Background()))

	// The handler processes each round in its own goroutine, so wait for
	// both rounds and assert order-independently.
	require.Eventually(t, func() bool {
		return len(notifier.snapshot()) == 2
	}, time.Second, 10*time.Millisecond)

	var gotRound1, gotRound2 bool
	for _, s := range notifier.snapshot() {
		if strings.Contains(s.PRDemoPath, "round1") {
			gotRound1 = true
		}
		if strings.Contains(s.PRDemoPath, "round2") {
			gotRound2 = true
		}
	}
	assert.True(t, gotRound1, "expected a round paired with round1 prdemo")
	assert.True(t, gotRound2, "expected a round paired with round2 prdemo")
}

func TestRecovery_ReplayUnnotified_RetriesAndMarksNotified(t *testing.T) {
	store := newTestStore(t)
	notifier := &mockRoundNotifier{}

	// Pre-populate state: a round that was uploaded but never notified.
	// "valid_summary.json" lives in testdata/ so ParseSummary can read it.
	roundKey := "20260503-000000.000"
	require.NoError(t, store.RecordUpload("server1", roundKey, UploadedArtifact{
		Filename:  "valid_summary.json",
		Type:      config.ArtifactTypeSummary,
		RemoteRef: "https://remote/summaries/valid_summary.json",
	}))
	require.NoError(t, store.RecordUpload("server1", roundKey, UploadedArtifact{
		Filename:  "round_001.prdemo",
		Type:      config.ArtifactTypePRDemo,
		RemoteRef: "https://remote/prdemos/round_001.prdemo",
	}))

	// Point summary location at testdata/ so the file is found by replay.
	cfg := config.ArtifactsConfig{
		config.ArtifactTypeSummary: config.Location{Location: "testdata", UploadPath: "summaries"},
		config.ArtifactTypePRDemo:  config.Location{Location: "/nonexistent", UploadPath: "prdemos"},
	}

	recovery := newTestRecovery(t, store, notifier, NewSummaryBuilder(testLogger(t), nil), cfg)

	err := recovery.ReplayUnnotified(context.Background(), time.Time{})
	require.NoError(t, err)

	require.Len(t, notifier.summaries, 1)
	s := notifier.summaries[0]
	assert.Equal(t, "muttrah_city_2", s.MapName)
	assert.Equal(t, "https://remote/summaries/valid_summary.json", s.RemoteRefs[config.ArtifactTypeSummary])

	records, err := store.QueryUnnotified("server1", time.Time{})
	require.NoError(t, err)
	assert.Empty(t, records)
}
