package log

import (
	"log/slog"

	"github.com/emilekm/artifacts-mover/internal/config"
)

func Error(err error) slog.Attr { return slog.String("error", err.Error()) }

func Path(path string) slog.Attr         { return slog.String("path", path) }
func ServerID(serverID string) slog.Attr { return slog.String("server_id", serverID) }
func RoundKey(roundKey string) slog.Attr { return slog.String("round_key", roundKey) }
func ArtifactType(typ config.ArtifactType) slog.Attr {
	return slog.String("artifact_type", typ.String())
}
