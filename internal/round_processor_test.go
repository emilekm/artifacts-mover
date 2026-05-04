package internal_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/emilekm/artifacts-mover/internal"
	"github.com/emilekm/artifacts-mover/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- hand-crafted mocks ---

type mockArtifactUploader struct {
	err      error
	uploaded []internal.Artifact
}

func (m *mockArtifactUploader) Upload(_ context.Context, a internal.Artifact) (internal.RemoteRef, error) {
	if m.err != nil {
		return "", m.err
	}
	m.uploaded = append(m.uploaded, a)
	return "remote/" + a.UploadPath + "/" + a.Path, nil
}

type mockRoundNotifier struct {
	err       error
	summaries []*internal.RoundSummary
}

func (m *mockRoundNotifier) Send(_ context.Context, s *internal.RoundSummary) error {
	if m.err != nil {
		return m.err
	}
	m.summaries = append(m.summaries, s)
	return nil
}

// --- helpers ---

var testArtifactsConfig = config.ArtifactsConfig{
	config.ArtifactTypePRDemo:  config.Location{Location: "/watch/prdemos", UploadPath: "prdemos"},
	config.ArtifactTypeSummary: config.Location{Location: "/watch/json", UploadPath: "summaries"},
}

// twoArtifactRound creates a round with a per-test temp copy of the summary
// fixture so that cleanupArtifacts doesn't delete the shared testdata file.
func twoArtifactRound(t *testing.T) internal.Round {
	t.Helper()
	dir := t.TempDir()
	summaryContent, err := os.ReadFile("testdata/valid_summary.json")
	require.NoError(t, err)
	summaryPath := filepath.Join(dir, "valid_summary.json")
	require.NoError(t, os.WriteFile(summaryPath, summaryContent, 0644))

	return internal.Round{
		config.ArtifactTypePRDemo: internal.Artifact{
			Path: "/watch/prdemos/round_001.prdemo",
			Type: config.ArtifactTypePRDemo,
		},
		config.ArtifactTypeSummary: internal.Artifact{
			Path: summaryPath,
			Type: config.ArtifactTypeSummary,
		},
	}
}

// --- tests ---

func TestRoundProcessor_ResolvesUploadPathFromConfig(t *testing.T) {
	store := newTestStore(t)
	uploader := &mockArtifactUploader{}
	notifier := &mockRoundNotifier{}

	proc := internal.NewRoundProcessor("server1", uploader, store, notifier, testArtifactsConfig)

	err := proc.Process(context.Background(), twoArtifactRound(t))
	require.NoError(t, err)

	require.Len(t, uploader.uploaded, 2)
	uploadedByPath := make(map[string]string)
	for _, a := range uploader.uploaded {
		uploadedByPath[a.UploadPath] = a.Path
	}
	assert.Contains(t, uploadedByPath, "prdemos")
	assert.Contains(t, uploadedByPath, "summaries")
}

func TestRoundProcessor_UploadFails_DoesNotNotify(t *testing.T) {
	store := newTestStore(t)
	uploader := &mockArtifactUploader{err: errors.New("scp: connection refused")}
	notifier := &mockRoundNotifier{}

	proc := internal.NewRoundProcessor("server1", uploader, store, notifier, testArtifactsConfig)

	err := proc.Process(context.Background(), twoArtifactRound(t))
	require.Error(t, err)

	assert.Empty(t, notifier.summaries)
}

func TestRoundProcessor_NotifyFails_UploadsStoredButRoundRemainsUnnotified(t *testing.T) {
	store := newTestStore(t)
	uploader := &mockArtifactUploader{}
	notifier := &mockRoundNotifier{err: errors.New("discord: rate limited")}

	proc := internal.NewRoundProcessor("server1", uploader, store, notifier, testArtifactsConfig)

	err := proc.Process(context.Background(), twoArtifactRound(t))
	require.Error(t, err)

	// round was uploaded but not notified → must appear in unnotified query
	records, err := store.QueryUnnotified("server1", time.Time{})
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Len(t, records[0].Artifacts, 2)
	assert.False(t, records[0].Notified)
}

func TestRoundProcessor_AllUploadsSucceed_NotifiesAndRecordsState(t *testing.T) {
	store := newTestStore(t)
	uploader := &mockArtifactUploader{}
	notifier := &mockRoundNotifier{}

	proc := internal.NewRoundProcessor("server1", uploader, store, notifier, testArtifactsConfig)

	err := proc.Process(context.Background(), twoArtifactRound(t))
	require.NoError(t, err)

	assert.Len(t, notifier.summaries, 1)

	// round was notified → must not appear in unnotified query
	records, err := store.QueryUnnotified("server1", time.Time{})
	require.NoError(t, err)
	assert.Empty(t, records)
}

func TestRoundProcessor_PassesRoundSummaryToNotifier(t *testing.T) {
	store := newTestStore(t)
	uploader := &mockArtifactUploader{}
	notifier := &mockRoundNotifier{}

	proc := internal.NewRoundProcessor("server1", uploader, store, notifier, testArtifactsConfig)

	err := proc.Process(context.Background(), twoArtifactRound(t))
	require.NoError(t, err)

	require.Len(t, notifier.summaries, 1)
	s := notifier.summaries[0]
	assert.Equal(t, "muttrah_city_2", s.MapName)
	assert.Equal(t, "gpm_cq", s.MapMode)
	assert.Equal(t, "/watch/prdemos/round_001.prdemo", s.PRDemoPath)
	assert.NotEmpty(t, s.RemoteRefs[config.ArtifactTypePRDemo])
	assert.NotEmpty(t, s.RemoteRefs[config.ArtifactTypeSummary])
}

func TestRoundProcessor_UploadOldFiles_PairsFilesByIndex(t *testing.T) {
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

	proc := internal.NewRoundProcessor("server1", &mockArtifactUploader{}, store, notifier, cfg)

	require.NoError(t, proc.UploadOldFiles(context.Background()))

	require.Len(t, notifier.summaries, 2)
	// Alphabetical sort guarantees round1 before round2.
	assert.Contains(t, notifier.summaries[0].PRDemoPath, "round1")
	assert.Contains(t, notifier.summaries[1].PRDemoPath, "round2")
}

func TestRoundProcessor_ReplayUnnotified_RetriesAndMarksNotified(t *testing.T) {
	store := newTestStore(t)
	notifier := &mockRoundNotifier{}

	// Pre-populate state: a round that was uploaded but never notified.
	// "valid_summary.json" lives in testdata/ so ParseSummary can read it.
	roundKey := "20260503-000000.000"
	require.NoError(t, store.RecordUpload("server1", roundKey, internal.UploadedArtifact{
		Filename:  "valid_summary.json",
		Type:      config.ArtifactTypeSummary,
		RemoteRef: "https://remote/summaries/valid_summary.json",
	}))
	require.NoError(t, store.RecordUpload("server1", roundKey, internal.UploadedArtifact{
		Filename:  "round_001.prdemo",
		Type:      config.ArtifactTypePRDemo,
		RemoteRef: "https://remote/prdemos/round_001.prdemo",
	}))

	// Point summary location at testdata/ so the file is found by replay.
	cfg := config.ArtifactsConfig{
		config.ArtifactTypeSummary: config.Location{Location: "testdata", UploadPath: "summaries"},
		config.ArtifactTypePRDemo:  config.Location{Location: "/nonexistent", UploadPath: "prdemos"},
	}

	proc := internal.NewRoundProcessor("server1", &mockArtifactUploader{}, store, notifier, cfg)

	err := proc.ReplayUnnotified(context.Background(), time.Time{})
	require.NoError(t, err)

	require.Len(t, notifier.summaries, 1)
	s := notifier.summaries[0]
	assert.Equal(t, "muttrah_city_2", s.MapName)
	assert.Equal(t, "https://remote/summaries/valid_summary.json", s.RemoteRefs[config.ArtifactTypeSummary])

	records, err := store.QueryUnnotified("server1", time.Time{})
	require.NoError(t, err)
	assert.Empty(t, records)
}

func TestRoundProcessor_CleanupAfterProcess_DeletesArtifacts(t *testing.T) {
	dir := t.TempDir()

	prDemoPath := filepath.Join(dir, "round_001.prdemo")
	summaryPath := filepath.Join(dir, "valid_summary.json")

	require.NoError(t, os.WriteFile(prDemoPath, []byte("prdemo"), 0644))
	summaryContent, err := os.ReadFile("testdata/valid_summary.json")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(summaryPath, summaryContent, 0644))

	store := newTestStore(t)
	notifier := &mockRoundNotifier{}

	cfg := config.ArtifactsConfig{
		config.ArtifactTypePRDemo:  config.Location{Location: dir, UploadPath: "prdemos"},
		config.ArtifactTypeSummary: config.Location{Location: dir, UploadPath: "summaries"},
	}

	proc := internal.NewRoundProcessor("server1", &mockArtifactUploader{}, store, notifier, cfg)

	round := internal.Round{
		config.ArtifactTypePRDemo:  internal.Artifact{Path: prDemoPath, Type: config.ArtifactTypePRDemo},
		config.ArtifactTypeSummary: internal.Artifact{Path: summaryPath, Type: config.ArtifactTypeSummary},
	}

	require.NoError(t, proc.Process(context.Background(), round))

	assert.NoFileExists(t, prDemoPath)
	assert.NoFileExists(t, summaryPath)
}

func TestRoundProcessor_CleanupAfterProcess_MovesArtifacts(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	prDemoPath := filepath.Join(srcDir, "round_001.prdemo")
	summaryPath := filepath.Join(srcDir, "valid_summary.json")

	require.NoError(t, os.WriteFile(prDemoPath, []byte("prdemo"), 0644))
	summaryContent, err := os.ReadFile("testdata/valid_summary.json")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(summaryPath, summaryContent, 0644))

	store := newTestStore(t)
	notifier := &mockRoundNotifier{}

	dstDirStr := dstDir
	cfg := config.ArtifactsConfig{
		config.ArtifactTypePRDemo:  config.Location{Location: srcDir, UploadPath: "prdemos", MovePath: &dstDirStr},
		config.ArtifactTypeSummary: config.Location{Location: srcDir, UploadPath: "summaries", MovePath: &dstDirStr},
	}

	proc := internal.NewRoundProcessor("server1", &mockArtifactUploader{}, store, notifier, cfg)

	round := internal.Round{
		config.ArtifactTypePRDemo:  internal.Artifact{Path: prDemoPath, Type: config.ArtifactTypePRDemo},
		config.ArtifactTypeSummary: internal.Artifact{Path: summaryPath, Type: config.ArtifactTypeSummary},
	}

	require.NoError(t, proc.Process(context.Background(), round))

	assert.NoFileExists(t, prDemoPath)
	assert.NoFileExists(t, summaryPath)
	assert.FileExists(t, filepath.Join(dstDir, "round_001.prdemo"))
	assert.FileExists(t, filepath.Join(dstDir, "valid_summary.json"))
}
