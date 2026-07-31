package internal

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/emilekm/artifacts-mover/internal/config"
)

//go:generate go tool go.uber.org/mock/mockgen -source=./handler.go -destination=./handler_mock.go -package=internal Notifier

type Notifier interface {
	Send(context.Context, *RoundSummary) error
}

type Round map[config.ArtifactType]Artifact

type Handler struct {
	logger       *slog.Logger
	process      func(Round)
	locToTyp     map[string]config.ArtifactType
	roundTimeout time.Duration

	typesCount int

	mu           sync.Mutex
	currentRound Round
	roundTimer   *time.Timer
	ctx          context.Context
	cancel       context.CancelFunc
}

func NewHandler(
	logger *slog.Logger,
	process func(Round),
	artifactsConfig config.ArtifactsConfig,
	roundTimeout time.Duration,
) (*Handler, error) {
	locToType := make(map[string]config.ArtifactType)

	for typ, location := range artifactsConfig {
		locToType[filepath.Clean(location.Location)] = typ
	}

	return &Handler{
		logger:       logger,
		process:      process,
		locToTyp:     locToType,
		roundTimeout: roundTimeout,
		typesCount:   len(locToType),
		currentRound: make(Round),
	}, nil
}

func (h *Handler) OnFileCreate(path string) {
	path = filepath.Clean(path)

	typ, ok := h.locToTyp[filepath.Dir(path)]
	if !ok {
		h.logger.LogAttrs(context.TODO(), slog.LevelWarn, "handler: no related type to path", slog.String("path", path))
		return
	}

	h.handleFile(Artifact{
		Path: path,
		Type: typ,
	})
}

func (h *Handler) handleFile(artifact Artifact) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.currentRound[artifact.Type]; ok {
		h.endCurrentRoundLocked()
	}
	if artifact.Type == config.ArtifactTypeBF2Demo && len(h.currentRound) > 0 {
		h.endCurrentRoundLocked()
	}

	h.currentRound[artifact.Type] = artifact

	if len(h.currentRound) == h.typesCount {
		h.endCurrentRoundLocked()
		return
	}

	if len(h.currentRound) == 1 && h.roundTimeout > 0 {
		h.startRoundTimer()
	}
}

func (h *Handler) startRoundTimer() {
	h.roundTimer = time.AfterFunc(h.roundTimeout, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if len(h.currentRound) > 0 {
			h.logger.LogAttrs(context.TODO(), slog.LevelWarn, "handler: round timeout reached", slog.Int("files", len(h.currentRound)))
			h.endCurrentRoundLocked()
		}
	})
}

func (h *Handler) endCurrentRoundLocked() {
	if h.roundTimer != nil {
		h.roundTimer.Stop()
		h.roundTimer = nil
	}

	if len(h.currentRound) == 0 {
		return
	}

	round := h.currentRound
	h.currentRound = make(Round)

	go h.process(round)
}

func (h *Handler) Close() {
	h.cancel()

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.roundTimer != nil {
		h.roundTimer.Stop()
		h.roundTimer = nil
	}
}

// move tries to rename source to destination.
// Falls back to copy+delete on cross-device moves.
func move(source, destination string) error {
	err := os.Rename(source, destination)
	if err != nil {
		return moveCrossDevice(source, destination)
	}
	return nil
}

func moveCrossDevice(source, destination string) error {
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	dst, err := os.Create(destination)
	if err != nil {
		src.Close()
		return err
	}
	_, err = io.Copy(dst, src)
	src.Close()
	dst.Close()
	if err != nil {
		return err
	}
	fi, err := src.Stat()
	if err != nil {
		if err := os.Remove(destination); err != nil {
			return err
		}
		return err
	}
	err = os.Chmod(destination, fi.Mode())
	if err != nil {
		if err := os.Remove(destination); err != nil {
			return err
		}
		return err
	}
	return os.Remove(source)
}
