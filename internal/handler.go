package internal

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/emilekm/artifacts-mover/internal/config"
)

//go:generate go run go.uber.org/mock/mockgen -source=./handler.go -destination=./handler_mock.go -package=internal Notifier

type Notifier interface {
	Send(context.Context, Round) error
}

type Round map[config.ArtifactType]Artifact

type Handler struct {
	uploader         Uploader
	notifier         Notifier
	artifactsConfig  config.ArtifactsConfig
	locToTyp         map[string]config.ArtifactType
	roundTimeout     time.Duration
	failedUploadPath string

	bf2DemoOnly bool
	typesCount  int

	mu           sync.Mutex
	currentRound Round
	roundTimer   *time.Timer
	ctx          context.Context
	cancel       context.CancelFunc
}

func NewHandler(
	uploader Uploader,
	notifier Notifier,
	artifactsConfig config.ArtifactsConfig,
	roundTimeout time.Duration,
	failedUploadPath string,
) (*Handler, error) {
	bf2DemoOnly := true

	locToType := make(map[string]config.ArtifactType)

	for typ, location := range artifactsConfig {
		locToType[filepath.Clean(location.Location)] = typ

		if typ != config.ArtifactTypeBF2Demo {
			bf2DemoOnly = false
		}
	}

	for _, artifact := range locToType {
		failedDir := filepath.Join(failedUploadPath, artifact.String())
		if err := os.MkdirAll(failedDir, 0755); err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Handler{
		uploader:         uploader,
		notifier:         notifier,
		artifactsConfig:  artifactsConfig,
		locToTyp:         locToType,
		roundTimeout:     roundTimeout,
		failedUploadPath: failedUploadPath,
		bf2DemoOnly:      bf2DemoOnly,
		typesCount:       len(locToType),
		currentRound:     make(Round),
		ctx:              ctx,
		cancel:           cancel,
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

	if len(h.currentRound) == 0 && h.roundTimeout > 0 && !h.bf2DemoOnly {
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
		select {
		case <-h.ctx.Done():
			return
		default:
		}

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

	err := h.uploader.Upload(h.currentRound)
	if err != nil {
		slog.Error("failed to upload round", "err", err, "op", "Handler.endCurrentRound")
		go h.backupFailedUploads(h.currentRound)
		return
	}

	go func(round Round) {
		if h.notifier != nil {
			err = h.notifier.Send(h.ctx, round)
			if err != nil {
				slog.Error("failed to send notification", "err", err, "op", "Handler.endCurrentRound")
			}
		}

		h.cleanupArtifacts(round)
	}(h.currentRound)

	h.currentRound = make(Round)
}

func (h *Handler) backupFailedUploads(round Round) {
	log := slog.With("op", "Handler.backupFailedUploads")

	for _, artifact := range round {
		newPath := filepath.Join(h.failedUploadPath, artifact.Type.String(), filepath.Base(artifact.Path))
		if err := os.Rename(artifact.Path, newPath); err != nil {
			log.Error("failed to move file", "src", artifact.Path, "dst", newPath, "err", err)
		}
	}
}

func (h *Handler) cleanupArtifacts(round Round) {
	log := slog.With("op", "Handler.cleanupArtifacts")

	for typ, artifact := range round {
		artifactConfig := h.artifactsConfig[typ]
		if artifactConfig.MovePath != nil {
			newPath := filepath.Join(*artifactConfig.MovePath, filepath.Base(artifact.Path))
			err := os.Rename(artifact.Path, newPath)
			if err != nil {
				log.Error("failed to move file", "path", artifact.Path, "err", err)
			}
		} else {
			if err := os.Remove(artifact.Path); err != nil {
				log.Error("failed to remove file", "path", artifact.Path, "err", err)
			}
		}
	}
}

func (h *Handler) UploadOldFiles() error {
	log := slog.With("op", "Handler.UploadOldFiles")

	allFiles := make(map[config.ArtifactType][]string)

	for path, typ := range h.locToTyp {
		var err error
		allFiles[typ], err = filepath.Glob(filepath.Join(path, "*"))
		if err != nil {
			return err
		}

		log.Debug("Found files", "path", path, "count", len(allFiles[typ]))
	}

	maxLen := 0
	for _, files := range allFiles {
		if len(files) > maxLen {
			maxLen = len(files)
		}
	}

	for i := range maxLen {
		for typ, files := range allFiles {
			if len(files) > i {
				log.Debug("Handling old file", "path", files[i], "type", typ.String())
				h.handleFile(Artifact{
					Path: files[i],
					Type: typ,
				})
			}
		}
	}

	return nil
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
