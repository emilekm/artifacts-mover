package internal

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	scp "github.com/bramvdbogaerde/go-scp"

	"github.com/emilekm/artifacts-mover/internal/config"
	"golang.org/x/crypto/ssh"
)

const (
	defaultConnTimeout = 20 * time.Second
)

type scpUploader struct {
	artifactsConfig config.ArtifactsConfig
	basePath        string
	address         string
	scpConf         *ssh.ClientConfig
}

func NewSCPUploader(
	conf config.SCPConfig,
	artifactsConfig config.ArtifactsConfig,
) (*scpUploader, error) {
	privKey, err := os.ReadFile(conf.PrivateKeyFile)
	if err != nil {
		return nil, err
	}

	scpConf, err := newSSHConfigFromPrivateKey(conf.Username, privKey)
	if err != nil {
		return nil, err
	}

	u := &scpUploader{
		artifactsConfig: artifactsConfig,
		basePath:        conf.BasePath,
		address:         conf.Address,
		scpConf:         scpConf,
	}

	return u, nil
}

func (u *scpUploader) Upload(round Round) error {
	log := slog.With("op", "scpUploader.Upload")

	client, err := u.connectSCPClient()
	if err != nil {
		return err
	}
	defer client.Close()

	ctx := context.Background()

	for typ, artifact := range round {
		file, err := os.Open(artifact.Path)
		if err != nil {
			return err
		}
		defer file.Close()

		err = client.CopyFromFile(
			ctx,
			*file,
			u.fullUploadPath(typ, artifact.Path),
			"0644",
		)
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

func (u *scpUploader) connectSCPClient() (*scp.Client, error) {
	client := scp.NewClient(u.address, u.scpConf)

	err := client.Connect()
	if err != nil {
		return nil, err
	}

	return &client, nil
}

func newSSHConfigFromPrivateKey(username string, privPEM []byte) (cfg *ssh.ClientConfig, err error) {
	var priv ssh.Signer
	priv, err = ssh.ParsePrivateKey(privPEM)
	if err != nil {
		return nil, err
	}

	cfg = &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(priv),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         defaultConnTimeout,
	}
	return
}
