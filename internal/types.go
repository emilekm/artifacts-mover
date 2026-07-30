package internal

import "github.com/emilekm/artifacts-mover/internal/config"

type Artifact struct {
	Path string
	Type config.ArtifactType
}

func (h *Handler) handleFile2(artifact Artifact) {
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
