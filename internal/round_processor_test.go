package internal_test

import (
	"context"
	"errors"
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
	summaries []internal.RoundSummary
}

func (m *mockRoundNotifier) Send(_ context.Context, s internal.RoundSummary) error {
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

func twoArtifactRound() internal.Round {
	return internal.Round{
		config.ArtifactTypePRDemo: internal.Artifact{
			Path: "/watch/prdemos/round_001.prdemo",
			Type: config.ArtifactTypePRDemo,
		},
		config.ArtifactTypeSummary: internal.Artifact{
			Path: "testdata/valid_summary.json",
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

	err := proc.Process(context.Background(), twoArtifactRound())
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

	err := proc.Process(context.Background(), twoArtifactRound())
	require.Error(t, err)

	assert.Empty(t, notifier.summaries)
}

func TestRoundProcessor_NotifyFails_UploadsStoredButRoundRemainsUnnotified(t *testing.T) {
	store := newTestStore(t)
	uploader := &mockArtifactUploader{}
	notifier := &mockRoundNotifier{err: errors.New("discord: rate limited")}

	proc := internal.NewRoundProcessor("server1", uploader, store, notifier, testArtifactsConfig)

	err := proc.Process(context.Background(), twoArtifactRound())
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

	err := proc.Process(context.Background(), twoArtifactRound())
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

	err := proc.Process(context.Background(), twoArtifactRound())
	require.NoError(t, err)

	require.Len(t, notifier.summaries, 1)
	s := notifier.summaries[0]
	assert.Equal(t, "muttrah_city_2", s.MapName)
	assert.Equal(t, "gpm_cq", s.MapMode)
	assert.Equal(t, "/watch/prdemos/round_001.prdemo", s.PRDemoPath)
	assert.NotEmpty(t, s.RemoteRefs[config.ArtifactTypePRDemo])
	assert.NotEmpty(t, s.RemoteRefs[config.ArtifactTypeSummary])
}
