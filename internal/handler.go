package internal

import (
	"log/slog"
	"path/filepath"

	"github.com/emilekm/artifacts-mover/internal/config"
)

type Round map[config.ArtifactType]string

type Handler struct {
	locToTyp   map[string]config.ArtifactType
	uploader   Uploader
	typesCount int

	currentRound Round
}

func NewHandler(uploader Uploader, artifactConf config.ArtifactsConfig) *Handler {
	locToType := make(map[string]config.ArtifactType)
	for typ, loc := range artifactConf {
		locToType[filepath.Clean(loc.Location)] = typ
	}

	return &Handler{
		locToTyp:     locToType,
		uploader:     uploader,
		typesCount:   len(locToType),
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

// shoudlEndRound is a function that determines if the current round should end
// It returns:
// -1 if the round should end without incoming file;
// 0 continue;
// 1 if the round should end with the incoming file;
// --
// only bf2demo
// upload previous file - aka upload when full
// --
// only others
// upload if files overlap
// upload when full with incoming
// --
// mixed
// upload if files overlap - something bad happened
// upload when full
func (h *Handler) shouldEndRound(incomingType config.ArtifactType) int {
	// Overlap check
	if _, ok := h.currentRound[incomingType]; ok {
		// We have all the files or something bad happened
		return -1
	}

	if len(h.currentRound) == h.typesCount-1 {
		// We have all but one file, so we can upload with it
		return 1
	}

	return 0
}

func (h *Handler) handleFile(path string, typ config.ArtifactType) {
	shoudlEnd := h.shouldEndRound(typ)
	if shoudlEnd == -1 {
		h.endCurrentRound()
	} else if shoudlEnd == 1 {
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
