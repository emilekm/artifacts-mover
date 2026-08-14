// Package integration wires the real ingest, store, upload and notify pieces
// together and drives them over files on disk. Only the uploader and notifier
// are stubbed, so the queue transitions (enqueue -> uploaded, enqueue ->
// published) run against a real bbolt store.
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
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	serverID       = "serverID"
	filenameLayout = "2006_01_02_15_04_05"

	// The workers poll once per second, so anything waiting on a queue
	// transition needs room for a couple of passes.
	settleTimeout = 5 * time.Second
	settleTick    = 25 * time.Millisecond

	// Long enough that no test hits the give-up path unless it asks to.
	defaultRetryWindow = time.Hour
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

		wantTypes     []types.ArtifactType
		wantUploaded  bool
		wantPublished bool
	}{
		{
			name:        "live rounds are uploaded and published",
			rounds:      2,
			withBF2Demo: true,
			wantTypes: []types.ArtifactType{
				types.ArtifactTypeBF2Demo, types.ArtifactTypePRDemo, types.ArtifactTypeSummary,
			},
			wantUploaded:  true,
			wantPublished: true,
		},
		{
			name:        "recovered rounds are uploaded and published",
			rounds:      2,
			recover:     true,
			withBF2Demo: true,
			wantTypes: []types.ArtifactType{
				types.ArtifactTypeBF2Demo, types.ArtifactTypePRDemo, types.ArtifactTypeSummary,
			},
			wantUploaded:  true,
			wantPublished: true,
		},
		{
			name:    "recovered round without bf2demo still completes",
			rounds:  1,
			recover: true,
			wantTypes: []types.ArtifactType{
				types.ArtifactTypePRDemo, types.ArtifactTypeSummary,
			},
			wantUploaded:  true,
			wantPublished: true,
		},
		{
			// Publication no longer waits for the upload: the links are built
			// from the local filenames and go live when the upload lands.
			name:        "upload failure does not hold up publication",
			rounds:      1,
			recover:     true,
			withBF2Demo: true,
			uploadErr:   assert.AnError,
			wantTypes: []types.ArtifactType{
				types.ArtifactTypeBF2Demo, types.ArtifactTypePRDemo, types.ArtifactTypeSummary,
			},
			wantPublished: true,
		},
		{
			name:        "notify failure leaves the round unpublished",
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
			}

			if test.wantPublished {
				require.Eventually(t, func() bool {
					return len(env.unpublished(t)) == 0
				}, settleTimeout, settleTick, "rounds should drain from the publish queue")
			} else {
				require.Eventually(t, func() bool {
					return len(env.notifier.calls()) >= 2
				}, settleTimeout, settleTick, "failing notifications should be retried")
				assert.Len(t, env.unpublished(t), test.rounds)
			}

			if test.wantPublished {
				published := make(map[string][]types.ArtifactType)
				for _, round := range env.notifier.published() {
					for typ := range round.Artifacts {
						published[round.RoundID] = append(published[round.RoundID], typ)
					}
				}
				require.Len(t, published, test.rounds)
				for _, roundID := range wantRoundIDs {
					types := published[roundID]
					slices.Sort(types)
					assert.Equal(t, test.wantTypes, types, "artifacts published for round %s", roundID)
				}
			}
		})
	}
}

// TestPublicationOrder pins the guarantee the whole design exists for: rounds
// reach the channel oldest first, whatever the uploads are doing.
func TestPublicationOrder(t *testing.T) {
	env := newEnv(t, assert.AnError, nil)

	var wantRoundIDs []string
	for i := range 3 {
		// The middle round has no bf2demo, so the scanner reconstructs it after
		// the other two and only the enqueue sort puts it back in place.
		wantRoundIDs = append(wantRoundIDs, env.writeRound(t, baseTime.Add(time.Duration(i)*time.Hour), i != 1))
	}

	_, err := env.scanner.Scan(t.Context())
	require.NoError(t, err)

	env.startWorkers(t)

	require.Eventually(t, func() bool {
		return len(env.unpublished(t)) == 0
	}, settleTimeout, settleTick, "every round should be published")

	assert.Equal(t, wantRoundIDs, env.notifier.publishedIDs(),
		"rounds must be published oldest first even while uploads keep failing")
}

// TestFailedRoundBlocksLaterRounds is the other half of the guarantee: a round
// that cannot be published must not let the next one take its place.
func TestFailedRoundBlocksLaterRounds(t *testing.T) {
	env := newEnv(t, nil, nil)

	first := env.writeRound(t, baseTime, true)
	second := env.writeRound(t, baseTime.Add(time.Hour), true)
	env.notifier.failRound(first, assert.AnError)

	_, err := env.scanner.Scan(t.Context())
	require.NoError(t, err)

	env.startWorkers(t)

	require.Eventually(t, func() bool {
		return len(env.notifier.calls()) >= 2
	}, settleTimeout, settleTick, "the failing round should be retried")
	assert.Empty(t, env.notifier.publishedIDs(), "no round may be published while an older one is failing")

	env.notifier.failRound(first, nil)

	require.Eventually(t, func() bool {
		return len(env.unpublished(t)) == 0
	}, settleTimeout, settleTick, "both rounds should publish once the older one recovers")
	assert.Equal(t, []string{first, second}, env.notifier.publishedIDs())
}

// TestGiveUpUnblocksLaterRounds covers the escape hatch: once a round has been
// failing for longer than the retry window it is published in degraded form and
// stops holding the channel.
func TestGiveUpUnblocksLaterRounds(t *testing.T) {
	env := newEnv(t, nil, nil)
	env.retryWindow = 10 * time.Millisecond

	first := env.writeRound(t, baseTime, true)
	second := env.writeRound(t, baseTime.Add(time.Hour), true)
	env.notifier.failRound(first, assert.AnError)

	_, err := env.scanner.Scan(t.Context())
	require.NoError(t, err)

	env.startWorkers(t)

	require.Eventually(t, func() bool {
		return len(env.unpublished(t)) == 0
	}, settleTimeout, settleTick, "giving up on the older round should release the newer one")

	assert.Equal(t, []string{first}, env.notifier.degradedIDs(),
		"only the round that ran out of time should be degraded")
	assert.Equal(t, []string{first, second}, env.notifier.publishedIDs(),
		"the degraded round keeps its place in the channel")
}

// TestScanEnqueuesChronologically guards the ordering the notify worker relies
// on while a scan is still running: a round enqueued later must never be older
// than one already enqueued.
func TestScanEnqueuesChronologically(t *testing.T) {
	env := newEnv(t, nil, nil)

	var wantRoundIDs []string
	for i := range 3 {
		wantRoundIDs = append(wantRoundIDs, env.writeRound(t, baseTime.Add(time.Duration(i)*time.Hour), i != 1))
	}

	storeMock := store.NewMockStore()
	var enqueued []string
	storeMock.On("EnqueueRound", mock.Anything).
		Run(func(args mock.Arguments) {
			enqueued = append(enqueued, args.Get(0).(types.Round).RoundID)
		}).
		Return(nil)

	scanner := ingest.NewScanner(testLogger(t), storeMock, env.handler, env.artifacts, serverID)
	_, err := scanner.Scan(t.Context())
	require.NoError(t, err)

	assert.Equal(t, wantRoundIDs, enqueued)
}

type env struct {
	dirs      map[types.ArtifactType]string
	artifacts config.ArtifactsConfig
	store     *store.BboltStore
	handler   *ingest.Handler
	scanner   *ingest.Scanner
	uploader  *stubUploader
	notifier  *stubNotifier

	retryWindow time.Duration
	briefing    time.Duration
	live        []string // paths to feed the handler, in creation order
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
		dirs:        dirs,
		artifacts:   artifactsConfig,
		store:       stateStore,
		handler:     handler,
		scanner:     ingest.NewScanner(logger, stateStore, handler, artifactsConfig, serverID),
		uploader:    &stubUploader{err: uploadErr},
		notifier:    newStubNotifier(notifyErr),
		retryWindow: defaultRetryWindow,
		briefing:    briefingTime(t, trackerFixture),
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
	notifyWorker := notify.NewWorker(testLogger(t), e.store, e.retryWindow)
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

func (e *env) unpublished(t *testing.T) []types.Round {
	rounds, err := e.store.UnpublishedRounds()
	require.NoError(t, err)
	return rounds[serverID]
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

type notifyCall struct {
	round    types.Round
	degraded bool
	err      error
}

type stubNotifier struct {
	mu          sync.Mutex
	err         error
	perRound    map[string]error
	notifyCalls []notifyCall
}

func newStubNotifier(err error) *stubNotifier {
	return &stubNotifier{err: err, perRound: make(map[string]error)}
}

// failRound overrides the stub's behaviour for one round; nil clears it.
func (n *stubNotifier) failRound(roundID string, err error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if err == nil {
		delete(n.perRound, roundID)
		return
	}
	n.perRound[roundID] = err
}

func (n *stubNotifier) Notify(_ context.Context, round types.Round) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	err := n.err
	if perRound, ok := n.perRound[round.RoundID]; ok {
		err = perRound
	}

	n.notifyCalls = append(n.notifyCalls, notifyCall{round: round, err: err})
	return err
}

func (n *stubNotifier) NotifyDegraded(_ context.Context, round types.Round) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.notifyCalls = append(n.notifyCalls, notifyCall{round: round, degraded: true})
	return nil
}

func (n *stubNotifier) calls() []notifyCall {
	n.mu.Lock()
	defer n.mu.Unlock()
	return slices.Clone(n.notifyCalls)
}

// published returns the rounds that made it into the channel, in the order they
// got there, degraded or not.
func (n *stubNotifier) published() []types.Round {
	var rounds []types.Round
	for _, call := range n.calls() {
		if call.err == nil {
			rounds = append(rounds, call.round)
		}
	}
	return rounds
}

func (n *stubNotifier) publishedIDs() []string {
	var ids []string
	for _, round := range n.published() {
		if !slices.Contains(ids, round.RoundID) {
			ids = append(ids, round.RoundID)
		}
	}
	return ids
}

func (n *stubNotifier) degradedIDs() []string {
	var ids []string
	for _, call := range n.calls() {
		if call.degraded {
			ids = append(ids, call.round.RoundID)
		}
	}
	return ids
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
