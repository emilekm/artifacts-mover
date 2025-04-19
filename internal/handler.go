package internal

import (
	"log/slog"
	"path/filepath"

	"github.com/emilekm/artifacts-mover/internal/config"
)

type Round map[config.ArtifactType]string

type Handler struct {
	locToTyp    map[string]config.ArtifactType
	uploader    uploader
	bf2DemoOnly bool
	typesCount  int

	currentRound Round
}

func NewHandler(uploader uploader, locToType map[string]config.ArtifactType) *Handler {
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
	path = filepath.Clean(path)
	typ, ok := h.locToTyp[filepath.Dir(path)]
	if !ok {
		slog.Error("unknown path", "path", path)
		return
	}

	h.handleFile(path, typ)
}

func (h *Handler) handleFile(path string, typ config.ArtifactType) {
	if _, ok := h.currentRound[typ]; ok {
		h.endCurrentRound()
	}

	if !h.bf2DemoOnly && len(h.currentRound) == h.typesCount-1 {
		h.currentRound[typ] = path
		h.endCurrentRound()
		return
	}

	h.currentRound[typ] = path
}

func (h *Handler) endCurrentRound() {
	_ = h.uploader.Upload(h.currentRound)
	h.currentRound = make(Round)
}

func (h *Handler) UploadOldFiles() error {
	allFiles := make(map[config.ArtifactType][]string)

	for path, typ := range h.locToTyp {
		var err error
		allFiles[typ], err = filepath.Glob(filepath.Join(path, "*"))
		if err != nil {
			return err
		}
	}

	// The number of files in each directory should be the same
	// or the first directory should have more files than the others

	maxLen := 0
	for _, files := range allFiles {
		if len(files) > maxLen {
			maxLen = len(files)
		}
	}

	for i := range maxLen {
		for typ, files := range allFiles {
			if len(files) > i {
				h.handleFile(files[i], typ)
			}
		}
	}

	return nil
}
