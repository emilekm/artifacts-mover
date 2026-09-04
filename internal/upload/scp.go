package upload

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/emilekm/artifacts-mover/internal/config"
	"github.com/emilekm/artifacts-mover/internal/types"
)

type scpUploader struct {
	basePath    string
	uploadPaths map[types.ArtifactType]string
	address     string
	privKeyFile string
	username    string
}

func NewSCPUploader(conf config.SCPConfig, uploadPaths map[types.ArtifactType]string) (*scpUploader, error) {
	return &scpUploader{
		basePath:    conf.BasePath,
		uploadPaths: uploadPaths,
		address:     conf.Address,
		username:    conf.Username,
		privKeyFile: conf.PrivateKeyFile,
	}, nil
}

func (u *scpUploader) Upload(ctx context.Context, artifact types.Artifact) error {
	uploadPath, ok := u.uploadPaths[artifact.Type]
	if !ok {
		return fmt.Errorf("scp_uploader: no upload path for artifact of type %q", artifact.Type)
	}

	remotePath := filepath.Join(u.basePath, uploadPath, filepath.Base(artifact.Path))
	dest := fmt.Sprintf("%s@%s:%s", u.username, u.address, remotePath)

	out, err := exec.CommandContext(ctx, "scp", "-B", "-i", u.privKeyFile, artifact.Path, dest).CombinedOutput()
	if err != nil {
		return fmt.Errorf("scp_uploader: error while calling scp: output: %q; error: %w", string(out), err)
	}

	return nil
}
