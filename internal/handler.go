package internal

import (
	"log/slog"
	"path/filepath"

	"github.com/emilekm/artifacts-mover/internal/config"
)

type Round map[config.ArtifactType]string

type Handler struct {
	LocToType   map[string]config.ArtifactType
	bf2DemoOnly bool
	uploader    Uploader
	typesCount  int

	currentRound Round
}

func NewHandler(uploader Uploader, artifactConf config.ArtifactsConfig) *Handler {
	bf2DemoOnly := true
	for typ := range artifactConf {
		if typ != config.ArtifactTypeBF2Demo {
			bf2DemoOnly = false
			break
		}
	}

	locToType := make(map[string]config.ArtifactType)
	for typ, loc := range artifactConf {
		locToType[filepath.Clean(loc.Location)] = typ
	}

	return &Handler{
		LocToType:    locToType,
		bf2DemoOnly:  bf2DemoOnly,
		uploader:     uploader,
		typesCount:   len(locToType),
		currentRound: make(Round),
	}
}

func (h *Handler) OnFileCreate(path string) {
	path = filepath.Clean(path)
	typ, ok := h.LocToType[filepath.Dir(path)]
	if !ok {
		slog.Error("unknown path", "path", path)
		return
	}

	h.handleFile(path, typ)
}

func (h *Handler) handleFile(path string, typ config.ArtifactType) {
	if len(h.currentRound) == h.typesCount {
		// This handles the case when we only have bf2demo files.
		h.endCurrentRound()
	}

	h.currentRound[typ] = path

	if h.bf2DemoOnly {
		// We only have BF2Demo files, so we'll upload it when new file comes in.
		return
	}

	if len(h.currentRound) == h.typesCount {
		// This handles the case when we have mixed or no bf2demos.
		h.endCurrentRound()
	}
}

func (h *Handler) endCurrentRound() {
	_ = h.uploader.Upload(h.currentRound)
	h.currentRound = make(Round)
}

func (h *Handler) UploadOldFiles() error {
	allFiles := make(map[config.ArtifactType][]string)

	for path, typ := range h.LocToType {
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
