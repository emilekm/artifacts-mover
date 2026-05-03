package internal

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/emilekm/artifacts-mover/internal/config"
)

const defaultConnTimeout = 20 * time.Second

type scpUploader struct {
	basePath    string
	address     string
	privKeyFile string
	username    string
}

func NewSCPUploader(conf config.SCPConfig) (*scpUploader, error) {
	return &scpUploader{
		basePath:    conf.BasePath,
		address:     conf.Address,
		username:    conf.Username,
		privKeyFile: conf.PrivateKeyFile,
	}, nil
}

func (u *scpUploader) Upload(ctx context.Context, artifact Artifact) (RemoteRef, error) {
	remotePath := filepath.Join(u.basePath, artifact.UploadPath, filepath.Base(artifact.Path))
	dest := fmt.Sprintf("%s@%s:%s", u.username, u.address, remotePath)

	out, err := exec.CommandContext(ctx, "scp", "-B", "-i", u.privKeyFile, artifact.Path, dest).CombinedOutput()
	if err != nil {
		slog.Debug("SCP command output", "output", string(out))
		return "", err
	}

	return dest, nil
}
