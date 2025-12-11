package internal

import (
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/emilekm/artifacts-mover/internal/config"
)

const (
	defaultConnTimeout = 20 * time.Second
)

type scpUploader struct {
	artifactsConfig config.ArtifactsConfig
	basePath        string
	address         string
	privKeyFile     string
	username        string
}

func NewSCPUploader(
	conf config.SCPConfig,
	artifactsConfig config.ArtifactsConfig,
) (*scpUploader, error) {
	u := &scpUploader{
		artifactsConfig: artifactsConfig,
		basePath:        conf.BasePath,
		address:         conf.Address,
		username:        conf.Username,
		privKeyFile:     conf.PrivateKeyFile,
	}

	return u, nil
}

func (u *scpUploader) Upload(round Round) error {
	log := slog.With("op", "scpUploader.Upload")

	for typ, artifact := range round {
		err := exec.Command("scp", "-F", u.privKeyFile, artifact.Path, fmt.Sprintf(
			"%s@%s:%s",
			u.address,
			u.username,
			u.fullUploadPath(typ, artifact.Path),
		)).Run()
		if err != nil {
			return err
		}
		log.Debug("uploaded file via SCP", "path", artifact.Path)
	}

	return nil
}

func (u *scpUploader) fullUploadPath(typ config.ArtifactType, path string) string {
	filename := filepath.Base(path)
	return filepath.Join(u.basePath, u.artifactsConfig[typ].UploadPath, filename)
}
