package log

import (
	"log/slog"

	"github.com/emilekm/artifacts-mover/internal/types"
)

func Error(err error) slog.Attr { return slog.String("error", err.Error()) }

func Path(path string) slog.Attr           { return slog.String("path", path) }
func ServerID(serverID string) slog.Attr   { return slog.String("server_id", serverID) }
func RoundID(roundID uint) slog.Attr       { return slog.Int("round_id", int(roundID)) }
func ArtifactID(artifactID uint) slog.Attr { return slog.Int("artifact_id", int(artifactID)) }
func ArtifactType(typ types.ArtifactType) slog.Attr {
	return slog.String("artifact_type", typ.String())
}
