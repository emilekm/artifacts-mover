package internal

import (
	"context"
	"fmt"
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
	process         func(Round)
	artifactsConfig config.ArtifactsConfig
	locToTyp        map[string]config.ArtifactType
	roundTimeout    time.Duration

	bf2DemoOnly bool
	typesCount  int

	mu           sync.Mutex
	currentRound Round
	roundTimer   *time.Timer
	ctx          context.Context
	cancel       context.CancelFunc
}

func NewHandler(
	process func(Round),
	artifactsConfig config.ArtifactsConfig,
	roundTimeout time.Duration,
) (*Handler, error) {
	bf2DemoOnly := true

	locToType := make(map[string]config.ArtifactType)

	for typ, location := range artifactsConfig {
		locToType[filepath.Clean(location.Location)] = typ

		if typ != config.ArtifactTypeBF2Demo {
			bf2DemoOnly = false
		}
	}

	return &Handler{
		process:         process,
		artifactsConfig: artifactsConfig,
		locToTyp:        locToType,
		roundTimeout:    roundTimeout,
		bf2DemoOnly:     bf2DemoOnly,
		typesCount:      len(locToType),
		currentRound:    make(Round),
	}, nil
}

func (h *Handler) OnFileCreate(path string) {
	log := slog.With("op", "Handler.OnFileCreate")

	path = filepath.Clean(path)

	log.Debug("Received file create event", "path", path)

	typ, ok := h.locToTyp[filepath.Dir(path)]
	if !ok {
		log.Error("No related type to path", "path", path)
		return
	}

	log.Debug(fmt.Sprintf("File type %s", typ), "path", path, "type", typ)

	h.handleFile(Artifact{
		Path: path,
		Type: typ,
	})
}

func (h *Handler) handleFile(artifact Artifact) {
	log := slog.With("op", "Handler.handleFile", "path", artifact.Path, "type", artifact.Type)

	log.Debug("Handling file")

	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.currentRound[artifact.Type]; ok {
		log.Debug("Type already in current round, ending")
		h.endCurrentRoundLocked()
	}

	if artifact.Type == config.ArtifactTypeBF2Demo && len(h.currentRound) > 0 {
		log.Debug("BF2 demo received, ending current round")
		h.endCurrentRoundLocked()
	}

	if len(h.currentRound) == 0 && h.roundTimeout > 0 {
		log.Debug("Starting round timeout", "timeout", h.roundTimeout)
		h.startRoundTimer()
	}

	if !h.bf2DemoOnly && len(h.currentRound) == h.typesCount-1 {
		log.Debug("All types except one in current round, ending")
		h.currentRound[artifact.Type] = artifact
		h.endCurrentRoundLocked()
		return
	}

	log.Debug("Adding artifact to current round")
	h.currentRound[artifact.Type] = artifact
}

func (h *Handler) startRoundTimer() {
	h.roundTimer = time.AfterFunc(h.roundTimeout, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if len(h.currentRound) > 0 {
			slog.Warn("Round timeout reached, ending incomplete round", "files", len(h.currentRound))
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
