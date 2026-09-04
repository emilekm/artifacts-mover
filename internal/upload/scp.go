package upload

import (
	"context"
	"fmt"
	"os"
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
	if err := trustHostKey(conf.Address); err != nil {
		return nil, fmt.Errorf("scp_uploader: error while trusting host key: %w", err)
	}

	return &scpUploader{
		basePath:    conf.BasePath,
		uploadPaths: uploadPaths,
		address:     conf.Address,
		username:    conf.Username,
		privKeyFile: conf.PrivateKeyFile,
	}, nil
}

// trustHostKey pins the host's SSH key to known_hosts via ssh-keyscan so
// scp does not prompt for (or fail on) host key confirmation.
func trustHostKey(address string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %w", err)
	}

	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return fmt.Errorf("could not create ssh directory: %w", err)
	}

	knownHostsFile := filepath.Join(sshDir, "known_hosts")
	f, err := os.OpenFile(knownHostsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("could not open known_hosts: %w", err)
	}
	defer f.Close()

	cmd := exec.Command("ssh-keyscan", "-H", address)
	cmd.Stdout = f
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("could not scan host key: %w", err)
	}

	return nil
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
