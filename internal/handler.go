package internal

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/emilekm/artifacts-mover/internal/config"
)

type Round map[config.ArtifactType]Artifact

type Handler struct {
	locToTyp    map[string]config.ArtifactType
	uploader    Uploader
	bf2DemoOnly bool
	typesCount  int

	currentRound Round
}

func NewHandler(uploader Uploader, locToType map[string]config.ArtifactType) *Handler {
	bf2DemoOnly := true
	for _, typ := range locToType {
		if typ != config.ArtifactTypeBF2Demo {
			bf2DemoOnly = false
			break
		}
	}

	return &Handler{
		locToTyp:     locToType,
		uploader:     uploader,
		typesCount:   len(locToType),
		bf2DemoOnly:  bf2DemoOnly,
		currentRound: make(Round),
	}
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

	if _, ok := h.currentRound[artifact.Type]; ok {
		log.Debug("Type already in current round, ending")
		h.endCurrentRound()
	}

	if !h.bf2DemoOnly && len(h.currentRound) == h.typesCount-1 {
		log.Debug("All types except one in current round, ending")
		h.currentRound[artifact.Type] = artifact
		h.endCurrentRound()
		return
	}

	log.Debug("Adding artifact to current round")
	h.currentRound[artifact.Type] = artifact
}

func (h *Handler) endCurrentRound() {
	err := h.uploader.Upload(h.currentRound)
	if err != nil {
		slog.Error("failed to upload round", "err", err, "op", "Handler.endCurrentRound")
	}
	h.currentRound = make(Round)
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
