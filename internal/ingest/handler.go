package ingest

import (
	"context"
	"database/sql"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/emilekm/artifacts-mover/internal/config"
	"github.com/emilekm/artifacts-mover/internal/db"
	"github.com/emilekm/artifacts-mover/internal/log"
	"github.com/emilekm/artifacts-mover/internal/types"
	"github.com/riverqueue/river"
)

const typesCount = 3

type Handler struct {
	logger       *slog.Logger
	db           *db.DB
	river        *river.Client[*sql.Tx]
	locToTyp     map[string]types.ArtifactType
	roundTimeout time.Duration
	serverID     string

	currentRound types.Round
	roundTimer   *time.Timer

	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
}

func NewHandler(
	ctx context.Context,
	logger *slog.Logger,
	db *db.DB,
	riverClient *river.Client[*sql.Tx],
	artifactsConfig config.ArtifactsConfig,
	roundTimeout time.Duration,
	serverID string,
) *Handler {
	locToType := make(map[string]types.ArtifactType)

	for typ, location := range artifactsConfig {
		locToType[filepath.Clean(location.Location)] = typ
	}

	ctx, cancel := context.WithCancel(ctx)
	h := &Handler{
		logger:       logger,
		db:           db,
		river:        riverClient,
		locToTyp:     locToType,
		roundTimeout: roundTimeout,
		currentRound: make(types.Round),
		ctx:          ctx,
		cancel:       cancel,
		serverID:     serverID,
	}

	if roundTimeout > 0 {
		h.roundTimer = time.AfterFunc(roundTimeout, func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			if len(h.currentRound) > 0 {
				h.logger.LogAttrs(h.ctx, slog.LevelWarn, "handler: round timeout reached", slog.Int("files", len(h.currentRound)))
				h.endCurrentRoundLocked()
			}
		})
		h.roundTimer.Stop()
	}

	return h
}

func (h *Handler) OnFileCreate(path string) {
	path = filepath.Clean(path)

	typ, ok := h.locToTyp[filepath.Dir(path)]
	if !ok {
		h.logger.LogAttrs(h.ctx, slog.LevelWarn, "handler: no related type to path", slog.String("path", path))
		return
	}

	h.handleFile(types.NewArtifact(path, typ))
}

func (h *Handler) handleFile(artifact types.Artifact) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.currentRound[artifact.Type]; ok {
		h.endCurrentRoundLocked()
	}
	if artifact.Type == types.ArtifactTypeBF2Demo && len(h.currentRound) > 0 {
		h.endCurrentRoundLocked()
	}

	h.currentRound[artifact.Type] = artifact

	if artifact.Type != types.ArtifactTypeBF2Demo && len(h.currentRound) == typesCount {
		h.endCurrentRoundLocked()
		return
	}

	if len(h.currentRound) == 1 {
		h.startRoundTimer()
	}
}

func (h *Handler) startRoundTimer() {
	if h.roundTimeout > 0 {
		h.roundTimer.Reset(h.roundTimeout)
	}
}

func (h *Handler) endCurrentRoundLocked() {
	if h.roundTimer != nil {
		h.roundTimer.Stop()
	}

	if len(h.currentRound) == 0 {
		return
	}

	round := h.currentRound
	h.currentRound = make(types.Round)

	timestamp := round[types.ArtifactTypePRDemo].Timestamp

	err := h.db.EnqueueRound(h.ctx, h.river, h.serverID, timestamp, round)
	if err != nil {
		h.logger.LogAttrs(
			h.ctx, slog.LevelError,
			"handler: failed to enqueue round",
			log.ServerID(h.serverID),
			log.Error(err),
		)
	}
}

func (h *Handler) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.roundTimer != nil {
		h.roundTimer.Stop()
	}

	h.cancel()
}
