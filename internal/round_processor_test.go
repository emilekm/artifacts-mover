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
	err    error
	rounds []internal.Round
}

func (m *mockRoundNotifier) Send(_ context.Context, r internal.Round) error {
	if m.err != nil {
		return m.err
	}
	m.rounds = append(m.rounds, r)
	return nil
}

// --- helpers ---

var testArtifactsConfig = config.ArtifactsConfig{
	config.ArtifactTypePRDemo:  config.Location{Location: "/watch/prdemos", UploadPath: "prdemos"},
	config.ArtifactTypeSummary: config.Location{Location: "/watch/json", UploadPath: "summaries"},
}

func singleArtifactRound() internal.Round {
	return internal.Round{
		config.ArtifactTypePRDemo: internal.Artifact{
			Path: "/watch/prdemos/round_001.prdemo",
			Type: config.ArtifactTypePRDemo,
		},
	}
}

// --- tests ---

func TestRoundProcessor_ResolvesUploadPathFromConfig(t *testing.T) {
	store := newTestStore(t)
	uploader := &mockArtifactUploader{}
	notifier := &mockRoundNotifier{}

	proc := internal.NewRoundProcessor("server1", uploader, store, notifier, testArtifactsConfig)

	err := proc.Process(context.Background(), singleArtifactRound())
	require.NoError(t, err)

	require.Len(t, uploader.uploaded, 1)
	assert.Equal(t, "prdemos", uploader.uploaded[0].UploadPath)
}

func TestRoundProcessor_UploadFails_DoesNotNotify(t *testing.T) {
	store := newTestStore(t)
	uploader := &mockArtifactUploader{err: errors.New("scp: connection refused")}
	notifier := &mockRoundNotifier{}

	proc := internal.NewRoundProcessor("server1", uploader, store, notifier, testArtifactsConfig)

	err := proc.Process(context.Background(), singleArtifactRound())
	require.Error(t, err)

	assert.Empty(t, notifier.rounds)
}

func TestRoundProcessor_NotifyFails_UploadsStoredButRoundRemainsUnnotified(t *testing.T) {
	store := newTestStore(t)
	uploader := &mockArtifactUploader{}
	notifier := &mockRoundNotifier{err: errors.New("discord: rate limited")}

	proc := internal.NewRoundProcessor("server1", uploader, store, notifier, testArtifactsConfig)

	err := proc.Process(context.Background(), singleArtifactRound())
	require.Error(t, err)

	// round was uploaded but not notified → must appear in unnotified query
	records, err := store.QueryUnnotified("server1", time.Time{})
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Len(t, records[0].Artifacts, 1)
	assert.False(t, records[0].Notified)
}

func TestRoundProcessor_AllUploadsSucceed_NotifiesAndRecordsState(t *testing.T) {
	store := newTestStore(t)
	uploader := &mockArtifactUploader{}
	notifier := &mockRoundNotifier{}

	proc := internal.NewRoundProcessor("server1", uploader, store, notifier, testArtifactsConfig)

	err := proc.Process(context.Background(), singleArtifactRound())
	require.NoError(t, err)

	assert.Len(t, notifier.rounds, 1)

	// round was notified → must not appear in unnotified query
	records, err := store.QueryUnnotified("server1", time.Time{})
	require.NoError(t, err)
	assert.Empty(t, records)
}
