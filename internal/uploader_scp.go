package internal

import (
	"os"
	"path/filepath"

	"github.com/emilekm/artifacts-mover/internal/config"
	"github.com/povsister/scp"
	"golang.org/x/crypto/ssh"
)

type scpUploader struct {
	artifactsConfig config.ArtifactsConfig

	basePath string
	address  string
	scpConf  *ssh.ClientConfig
}

func NewSCPUploader(
	conf config.SCPConfig,
	artifactsConfig config.ArtifactsConfig,
) (*scpUploader, error) {
	privKey, err := os.ReadFile(conf.PrivateKeyFile)
	if err != nil {
		return nil, err
	}

	scpConf, err := scp.NewSSHConfigFromPrivateKey(conf.Username, privKey)
	if err != nil {
		return nil, err
	}

	u := &scpUploader{
		artifactsConfig: artifactsConfig,
		basePath:        conf.BasePath,
		address:         conf.Address,
		scpConf:         scpConf,
	}

	_, err = u.client()
	if err != nil {
		return nil, err
	}

	return u, nil
}

func (u *scpUploader) Upload(round Round) error {
	client, err := u.client()
	if err != nil {
		return err
	}

	defer client.Close()

	for typ, artifact := range round {
		err := client.CopyFileToRemote(
			artifact.Path,
			u.fullUploadPath(typ, artifact.Path),
			&scp.FileTransferOption{},
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (u *scpUploader) fullUploadPath(typ config.ArtifactType, path string) string {
	filename := filepath.Base(path)
	return filepath.Join(u.basePath, u.artifactsConfig[typ].UploadPath, filename)
}

func (u *scpUploader) client() (*scp.Client, error) {
	return scp.NewClient(u.address, u.scpConf, &scp.ClientOption{})
}
