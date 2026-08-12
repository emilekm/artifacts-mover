// Package integration wires the real ingest, store, upload and notify pieces
// together and drives them over files on disk. Only the uploader and notifier
// are stubbed, so the queue transitions (enqueue -> uploaded -> notified) run
// against a real bbolt store.
package integration

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/emilekm/artifacts-mover/internal/config"
	"github.com/emilekm/artifacts-mover/internal/ingest"
	"github.com/emilekm/artifacts-mover/internal/notify"
	"github.com/emilekm/artifacts-mover/internal/store"
	"github.com/emilekm/artifacts-mover/internal/types"
	"github.com/emilekm/artifacts-mover/internal/upload"
	"github.com/emilekm/go-prbf2/prdemo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	serverID       = "serverID"
	filenameLayout = "2006_01_02_15_04_05"

	// The workers poll once per second, so anything waiting on a queue
	// transition needs room for a couple of passes.
	settleTimeout = 5 * time.Second
	settleTick    = 25 * time.Millisecond
)

var (
	trackerFixture = filepath.Join(".", "testdata", "tracker_2026_06_27_10_33_34_kokan_gpm_skirmish_64.PRdemo")
	summaryFixture = filepath.Join(".", "testdata", "valid_summary.json")

	// baseTime matches the fixture's own filename; rounds are spaced an hour
	// apart so no bf2demo can match a neighbouring prdemo.
	baseTime = time.Date(2026, 6, 27, 10, 33, 34, 0, time.UTC)
)

func TestQueueLifecycle(t *testing.T) {
	tests := []struct {
		name        string
		rounds      int
		recover     bool
		withBF2Demo bool
		uploadErr   error
		notifyErr   error

		wantTypes    []types.ArtifactType
		wantUploaded bool
		wantNotified bool
	}{
		{
			name:        "live rounds are uploaded and notified",
			rounds:      2,
			withBF2Demo: true,
			wantTypes: []types.ArtifactType{
				types.ArtifactTypeBF2Demo, types.ArtifactTypePRDemo, types.ArtifactTypeSummary,
			},
			wantUploaded: true,
			wantNotified: true,
		},
		{
			name:        "recovered rounds are uploaded and notified",
			rounds:      2,
			recover:     true,
			withBF2Demo: true,
			wantTypes: []types.ArtifactType{
				types.ArtifactTypeBF2Demo, types.ArtifactTypePRDemo, types.ArtifactTypeSummary,
			},
			wantUploaded: true,
			wantNotified: true,
		},
		{
			name:    "recovered round without bf2demo still completes",
			rounds:  1,
			recover: true,
			wantTypes: []types.ArtifactType{
				types.ArtifactTypePRDemo, types.ArtifactTypeSummary,
			},
			wantUploaded: true,
			wantNotified: true,
		},
		{
			name:        "upload failure leaves the round pending upload",
			rounds:      1,
			recover:     true,
			withBF2Demo: true,
			uploadErr:   assert.AnError,
			wantTypes: []types.ArtifactType{
				types.ArtifactTypeBF2Demo, types.ArtifactTypePRDemo, types.ArtifactTypeSummary,
			},
		},
		{
			name:        "notify failure leaves the round pending notification",
			rounds:      1,
			recover:     true,
			withBF2Demo: true,
			notifyErr:   assert.AnError,
			wantTypes: []types.ArtifactType{
				types.ArtifactTypeBF2Demo, types.ArtifactTypePRDemo, types.ArtifactTypeSummary,
			},
			wantUploaded: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env := newEnv(t, test.uploadErr, test.notifyErr)

			var wantRoundIDs []string
			for i := range test.rounds {
				wantRoundIDs = append(wantRoundIDs, env.writeRound(t, baseTime.Add(time.Duration(i)*time.Hour), test.withBF2Demo))
			}

			if test.recover {
				consumed, err := env.scanner.Scan(t.Context())
				require.NoError(t, err)
				require.Len(t, consumed, test.rounds*len(test.wantTypes))
			} else {
				env.replayLive(t)
			}

			require.ElementsMatch(t, wantRoundIDs, env.enqueuedRoundIDs(t),
				"every round should be in the store before the workers run")

			env.startWorkers(t)

			if test.wantUploaded {
				require.Eventually(t, func() bool {
					return len(env.pendingUploads(t)) == 0
				}, settleTimeout, settleTick, "rounds should drain from the upload queue")
			} else {
				// Nothing marks the round uploaded, so it is retried forever;
				// waiting for a second attempt proves the first one failed.
				require.Eventually(t, func() bool {
					return len(env.uploader.calls()) >= 2
				}, settleTimeout, settleTick, "failing uploads should be retried")
				assert.Len(t, env.pendingUploads(t), test.rounds)
				assert.Empty(t, env.notifier.calls(), "an unuploaded round must not be notified")
			}

			if test.wantNotified {
				require.Eventually(t, func() bool {
					return len(env.pendingNotifications(t)) == 0
				}, settleTimeout, settleTick, "rounds should drain from the notify queue")
			} else if test.wantUploaded {
				require.Eventually(t, func() bool {
					return len(env.notifier.calls()) >= 2
				}, settleTimeout, settleTick, "failing notifications should be retried")
				assert.Len(t, env.pendingNotifications(t), test.rounds)
			}

			if test.wantNotified {
				notified := make(map[string][]types.ArtifactType)
				for _, round := range env.notifier.calls() {
					for typ := range round.Artifacts {
						notified[round.RoundID] = append(notified[round.RoundID], typ)
					}
				}
				require.Len(t, notified, test.rounds)
				for _, roundID := range wantRoundIDs {
					types := notified[roundID]
					slices.Sort(types)
					assert.Equal(t, test.wantTypes, types, "artifacts notified for round %s", roundID)
				}
			}
		})
	}
}

type env struct {
	dirs     map[types.ArtifactType]string
	store    *store.BboltStore
	handler  *ingest.Handler
	scanner  *ingest.Scanner
	uploader *stubUploader
	notifier *stubNotifier

	briefing time.Duration
	live     []string // paths to feed the handler, in creation order
}

func newEnv(t *testing.T, uploadErr, notifyErr error) *env {
	t.Helper()

	root := t.TempDir()
	dirs := map[types.ArtifactType]string{
		types.ArtifactTypeBF2Demo: filepath.Join(root, "bf2demos"),
		types.ArtifactTypePRDemo:  filepath.Join(root, "prdemos"),
		types.ArtifactTypeSummary: filepath.Join(root, "json"),
	}
	artifactsConfig := config.ArtifactsConfig{}
	for typ, dir := range dirs {
		require.NoError(t, os.MkdirAll(dir, 0o750))
		artifactsConfig[typ] = config.Location{Location: dir}
	}

	stateStore, err := store.NewBboltStore(filepath.Join(root, "state.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, stateStore.Close()) })

	logger := testLogger(t)
	handler := ingest.NewHandler(t.Context(), logger, stateStore, artifactsConfig, time.Hour, serverID)
	t.Cleanup(handler.Close)

	return &env{
		dirs:     dirs,
		store:    stateStore,
		handler:  handler,
		scanner:  ingest.NewScanner(logger, stateStore, handler, artifactsConfig, serverID),
		uploader: &stubUploader{err: uploadErr},
		notifier: &stubNotifier{err: notifyErr},
		briefing: briefingTime(t, trackerFixture),
	}
}

// writeRound lays down one round's files and returns the round ID the ingest
// side is expected to derive from them. The bf2demo is created briefing seconds
// after the prdemo, which is the window matchRounds accepts.
func (e *env) writeRound(t *testing.T, ts time.Time, withBF2Demo bool) string {
	t.Helper()

	stamp := ts.Format(filenameLayout)

	if withBF2Demo {
		bf2demo := filepath.Join(e.dirs[types.ArtifactTypeBF2Demo],
			"bf2_"+ts.Add(e.briefing).Format(filenameLayout)+".bf2demo")
		require.NoError(t, os.WriteFile(bf2demo, nil, 0o600))
		e.live = append(e.live, bf2demo)
	}

	prdemoPath := filepath.Join(e.dirs[types.ArtifactTypePRDemo],
		"tracker_"+stamp+"_kokan_gpm_skirmish_64.PRdemo")
	copyFile(t, trackerFixture, prdemoPath)
	e.live = append(e.live, prdemoPath)

	summary := filepath.Join(e.dirs[types.ArtifactTypeSummary], "summary_"+stamp+".json")
	copyFile(t, summaryFixture, summary)
	e.live = append(e.live, summary)

	return ts.Format(time.RFC3339)
}

// replayLive feeds the files to the handler in the order they were created,
// the way the watcher would during normal running.
func (e *env) replayLive(t *testing.T) {
	t.Helper()
	for _, path := range e.live {
		e.handler.OnFileCreate(path)
	}
}

func (e *env) startWorkers(t *testing.T) {
	t.Helper()

	uploadWorker := upload.NewWorker(testLogger(t), e.store)
	uploadWorker.Register(serverID, e.uploader)
	notifyWorker := notify.NewWorker(testLogger(t), e.store)
	notifyWorker.Register(serverID, e.notifier)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = uploadWorker.Watch(t.Context()) }()
	go func() { defer wg.Done(); _ = notifyWorker.Watch(t.Context()) }()
	t.Cleanup(wg.Wait)
}

func (e *env) pendingUploads(t *testing.T) []types.Round {
	rounds, err := e.store.PendingUploads()
	require.NoError(t, err)
	return rounds
}

func (e *env) pendingNotifications(t *testing.T) []types.Round {
	rounds, err := e.store.PendingNotifications()
	require.NoError(t, err)
	return rounds
}

// enqueuedRoundIDs reads the queue before any upload has happened, when every
// round is still pending upload.
func (e *env) enqueuedRoundIDs(t *testing.T) []string {
	t.Helper()
	var ids []string
	for _, round := range e.pendingUploads(t) {
		ids = append(ids, round.RoundID)
	}
	return ids
}

type stubUploader struct {
	mu        sync.Mutex
	err       error
	artifacts []types.Artifact
}

func (u *stubUploader) Upload(_ context.Context, artifact types.Artifact) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.artifacts = append(u.artifacts, artifact)
	return u.err
}

func (u *stubUploader) calls() []types.Artifact {
	u.mu.Lock()
	defer u.mu.Unlock()
	return slices.Clone(u.artifacts)
}

type stubNotifier struct {
	mu     sync.Mutex
	err    error
	rounds []types.Round
}

func (n *stubNotifier) Notify(_ context.Context, round types.Round) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.rounds = append(n.rounds, round)
	return n.err
}

func (n *stubNotifier) calls() []types.Round {
	n.mu.Lock()
	defer n.mu.Unlock()
	return slices.Clone(n.rounds)
}

// briefingTime reads the preround length out of the tracker, which is the
// offset the scanner expects between a prdemo and its bf2demo.
func briefingTime(t *testing.T, path string) time.Duration {
	t.Helper()

	reader, err := prdemo.NewDemoReaderFromFile(path)
	require.NoError(t, err)

	for reader.Next() {
		msg, err := reader.GetMessage()
		require.NoError(t, err)
		if msg.Type != prdemo.ServerDetailsType {
			continue
		}
		var details prdemo.ServerDetails
		require.NoError(t, msg.Decode(&details))
		return time.Duration(details.BriefingTime) * time.Second
	}

	t.Fatalf("no ServerDetails message in %s", path)
	return 0
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	contents, err := os.ReadFile(src)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(dst, contents, 0o600))
}

func testLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(t.Output(), &slog.HandlerOptions{AddSource: true}))
}
