package upload

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/emilekm/artifacts-mover/internal/store"
	"github.com/emilekm/artifacts-mover/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const registeredServer = "serverID"

type mockUploader struct {
	mock.Mock
}

func (u *mockUploader) Upload(ctx context.Context, artifact types.Artifact) error {
	return u.Called(ctx, artifact).Error(0)
}

func TestWorkerUploadRecord(t *testing.T) {
	bf2demo := types.Artifact{Type: types.ArtifactTypeBF2Demo, Path: "bf2demos/file1"}
	prdemo := types.Artifact{Type: types.ArtifactTypePRDemo, Path: "prdemos/file1"}
	uploadedPRDemo := types.Artifact{Type: types.ArtifactTypePRDemo, Path: "prdemos/file1", Uploaded: true}

	tests := []struct {
		name        string
		serverID    string
		artifacts   []types.Artifact
		uploadErrs  map[types.ArtifactType]error
		markErrs    map[types.ArtifactType]error
		wantUploads []types.ArtifactType
		wantMarks   []types.ArtifactType
	}{
		{
			name:        "marks every pending artifact as uploaded",
			serverID:    registeredServer,
			artifacts:   []types.Artifact{bf2demo, prdemo},
			wantUploads: []types.ArtifactType{types.ArtifactTypeBF2Demo, types.ArtifactTypePRDemo},
			wantMarks:   []types.ArtifactType{types.ArtifactTypeBF2Demo, types.ArtifactTypePRDemo},
		},
		{
			name:        "skips artifacts already uploaded",
			serverID:    registeredServer,
			artifacts:   []types.Artifact{bf2demo, uploadedPRDemo},
			wantUploads: []types.ArtifactType{types.ArtifactTypeBF2Demo},
			wantMarks:   []types.ArtifactType{types.ArtifactTypeBF2Demo},
		},
		{
			name:      "does not mark uploaded when the upload fails",
			serverID:  registeredServer,
			artifacts: []types.Artifact{bf2demo, prdemo},
			uploadErrs: map[types.ArtifactType]error{
				types.ArtifactTypeBF2Demo: errors.New("connection refused"),
			},
			// The failed artifact is not recorded, but it does not stop the others.
			wantUploads: []types.ArtifactType{types.ArtifactTypeBF2Demo, types.ArtifactTypePRDemo},
			wantMarks:   []types.ArtifactType{types.ArtifactTypePRDemo},
		},
		{
			name:      "keeps going when the store rejects the mark",
			serverID:  registeredServer,
			artifacts: []types.Artifact{bf2demo, prdemo},
			markErrs: map[types.ArtifactType]error{
				types.ArtifactTypeBF2Demo: errors.New("store unavailable"),
				types.ArtifactTypePRDemo:  errors.New("store unavailable"),
			},
			wantUploads: []types.ArtifactType{types.ArtifactTypeBF2Demo, types.ArtifactTypePRDemo},
			wantMarks:   []types.ArtifactType{types.ArtifactTypeBF2Demo, types.ArtifactTypePRDemo},
		},
		{
			name:      "ignores rounds from unregistered servers",
			serverID:  "unknownServer",
			artifacts: []types.Artifact{bf2demo},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			storeMock := store.NewMockStore()
			uploaderMock := &mockUploader{}

			round := testRound(test.serverID, "roundID", test.artifacts...)
			seedRound(t, storeMock, round)

			for _, typ := range test.wantUploads {
				uploaderMock.On("Upload", mock.Anything, round.Artifacts[typ]).
					Return(test.uploadErrs[typ]).Once()
			}
			for _, typ := range test.wantMarks {
				storeMock.On("MarkUploaded", test.serverID, "roundID", typ).
					Return(test.markErrs[typ]).Once()
			}

			newTestWorker(t, storeMock, uploaderMock).uploadRecord(t.Context(), round)

			// Missing calls fail AssertExpectations; unexpected ones panic in
			// testify, so the counts pin the behaviour from both sides.
			uploaderMock.AssertExpectations(t)
			storeMock.AssertExpectations(t)
			uploaderMock.AssertNumberOfCalls(t, "Upload", len(test.wantUploads))
			storeMock.AssertNumberOfCalls(t, "MarkUploaded", len(test.wantMarks))
		})
	}
}

// Watch stays as separate cases: each drives cancellation from a different
// call, so there is no shared table shape to extract.
func TestWorkerWatch(t *testing.T) {
	t.Run("uploads pending rounds until the context is cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		storeMock := store.NewMockStore()
		uploaderMock := &mockUploader{}

		round := testRound(registeredServer, "roundID",
			types.Artifact{Type: types.ArtifactTypeBF2Demo, Path: "bf2demos/file1"},
		)
		seedRound(t, storeMock, round)

		storeMock.On("PendingUploads").Return(nil)
		storeMock.On("MarkUploaded", registeredServer, "roundID", types.ArtifactTypeBF2Demo).Return(nil).Once()
		uploaderMock.On("Upload", mock.Anything, mock.Anything).
			Run(func(mock.Arguments) { cancel() }).
			Return(nil).
			Once()

		err := newTestWorker(t, storeMock, uploaderMock).Watch(ctx)

		assert.ErrorIs(t, err, context.Canceled)
		uploaderMock.AssertExpectations(t)
		storeMock.AssertExpectations(t)
	})

	t.Run("keeps polling when the store returns an error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		storeMock := store.NewMockStore()
		uploaderMock := &mockUploader{}

		storeMock.On("PendingUploads").
			Run(func(mock.Arguments) { cancel() }).
			Return(errors.New("store unavailable"))

		err := newTestWorker(t, storeMock, uploaderMock).Watch(ctx)

		assert.ErrorIs(t, err, context.Canceled)
		uploaderMock.AssertNotCalled(t, "Upload")
	})
}

func newTestWorker(t *testing.T, stateStore *store.MockStore, uploader Uploader) *Worker {
	t.Helper()
	worker := NewWorker(testLogger(t), stateStore)
	worker.Register(registeredServer, uploader)
	return worker
}

func testRound(serverID, roundID string, artifacts ...types.Artifact) types.Round {
	round := types.NewRound(serverID)
	round.RoundID = roundID
	for _, artifact := range artifacts {
		round.Artifacts[artifact.Type] = artifact
	}
	return *round
}

// seedRound records the round in the store so MarkUploaded finds it.
func seedRound(t *testing.T, stateStore *store.MockStore, round types.Round) {
	t.Helper()
	stateStore.On("EnqueueRound", round).Return(nil).Once()
	require.NoError(t, stateStore.EnqueueRound(round))
}

func testLogger(t *testing.T) *slog.Logger {
	t.Helper()
	opts := &slog.HandlerOptions{
		AddSource: true,
	}
	return slog.New(slog.NewTextHandler(t.Output(), opts))
}
