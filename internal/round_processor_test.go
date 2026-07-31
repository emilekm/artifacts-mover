package internal

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/emilekm/artifacts-mover/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- hand-crafted mocks ---

type mockArtifactUploader struct {
	mu       sync.Mutex
	err      error
	uploaded []Artifact
}

func (m *mockArtifactUploader) Upload(_ context.Context, a Artifact) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.uploaded = append(m.uploaded, a)
	return nil
}

type mockRoundNotifier struct {
	mu        sync.Mutex
	err       error
	summaries []*RoundSummary
}

func (m *mockRoundNotifier) Send(_ context.Context, s *RoundSummary) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.summaries = append(m.summaries, s)
	return nil
}

func (m *mockRoundNotifier) snapshot() []*RoundSummary {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]*RoundSummary(nil), m.summaries...)
}

// --- helpers ---

var testArtifactsConfig = config.ArtifactsConfig{
	config.ArtifactTypePRDemo:  config.Location{Location: "/watch/prdemos", UploadPath: "prdemos"},
	config.ArtifactTypeSummary: config.Location{Location: "/watch/json", UploadPath: "summaries"},
}

var testDiscordURLs = map[string]string{
	"prdemo":  "https://cdn.example.com/prdemos/%s",
	"summary": "https://cdn.example.com/summaries/%s",
}

// twoArtifactRound creates a round with a per-test temp copy of the summary
// fixture so that cleanupArtifacts doesn't delete the shared testdata file.
func twoArtifactRound(t *testing.T) Round {
	t.Helper()
	dir := t.TempDir()
	summaryContent, err := os.ReadFile("testdata/valid_summary.json")
	require.NoError(t, err)
	summaryPath := filepath.Join(dir, "valid_summary.json")
	require.NoError(t, os.WriteFile(summaryPath, summaryContent, 0644))

	return Round{
		config.ArtifactTypePRDemo: Artifact{
			Path: "/watch/prdemos/round_001.prdemo",
			Type: config.ArtifactTypePRDemo,
		},
		config.ArtifactTypeSummary: Artifact{
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

	proc := NewRoundProcessor(testLogger(t), "server1", uploader, store, notifier, NewSummaryBuilder(testLogger(t), nil), testArtifactsConfig)

	proc.Process(context.Background(), twoArtifactRound(t))

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

	proc := NewRoundProcessor(testLogger(t), "server1", uploader, store, notifier, NewSummaryBuilder(testLogger(t), nil), testArtifactsConfig)

	proc.Process(context.Background(), twoArtifactRound(t))

	assert.Empty(t, notifier.summaries)
}

func TestRoundProcessor_NotifyFails_UploadsStoredButRoundRemainsUnnotified(t *testing.T) {
	store := newTestStore(t)
	uploader := &mockArtifactUploader{}
	notifier := &mockRoundNotifier{err: errors.New("discord: rate limited")}

	proc := NewRoundProcessor(testLogger(t), "server1", uploader, store, notifier, NewSummaryBuilder(testLogger(t), nil), testArtifactsConfig)

	proc.Process(context.Background(), twoArtifactRound(t))

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

	proc := NewRoundProcessor(testLogger(t), "server1", uploader, store, notifier, NewSummaryBuilder(testLogger(t), nil), testArtifactsConfig)

	proc.Process(context.Background(), twoArtifactRound(t))

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

	proc := NewRoundProcessor(testLogger(t), "server1", uploader, store, notifier, NewSummaryBuilder(testLogger(t), testDiscordURLs), testArtifactsConfig)

	proc.Process(context.Background(), twoArtifactRound(t))

	require.Len(t, notifier.summaries, 1)
	s := notifier.summaries[0]
	assert.Equal(t, "muttrah_city_2", s.MapName)
	assert.Equal(t, "gpm_cq", s.MapMode)
	assert.Equal(t, "/watch/prdemos/round_001.prdemo", s.PRDemoPath)
	assert.NotEmpty(t, s.RemoteRefs[config.ArtifactTypePRDemo])
	assert.NotEmpty(t, s.RemoteRefs[config.ArtifactTypeSummary])
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

	proc := NewRoundProcessor(testLogger(t), "server1", &mockArtifactUploader{}, store, notifier, NewSummaryBuilder(testLogger(t), nil), cfg)

	round := Round{
		config.ArtifactTypePRDemo:  Artifact{Path: prDemoPath, Type: config.ArtifactTypePRDemo},
		config.ArtifactTypeSummary: Artifact{Path: summaryPath, Type: config.ArtifactTypeSummary},
	}

	proc.Process(context.Background(), round)

	assert.NoFileExists(t, prDemoPath)
	assert.NoFileExists(t, summaryPath)
}
