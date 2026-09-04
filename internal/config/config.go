package config

import (
	"os"
	"time"

	"github.com/emilekm/artifacts-mover/internal/types"
	"github.com/goccy/go-yaml"
)

type BasicAuth struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type SCPConfig struct {
	Address        string `yaml:"address"`
	Username       string `yaml:"username"`
	PrivateKeyFile string `yaml:"privateKeyFile"`
	BasePath       string `yaml:"basePath"`
}

type HTTPSAuth struct {
	Basic   *BasicAuth        `yaml:"basic,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty"`
}

type HTTPSConfig struct {
	URL  string    `yaml:"url"`
	Auth HTTPSAuth `yaml:"auth"`
}

// TODO: Implement SFTP
// type SFTPConfig struct {
// }

type UploadConfig struct {
	SCP   *SCPConfig   `yaml:"scp,omitempty"`
	HTTPS *HTTPSConfig `yaml:"https,omitempty"`
	// SFTP  *SFTPConfig  `yaml:"sftp,omitempty"`
}

type Location struct {
	Location   string `yaml:"location"`
	UploadPath string `yaml:"uploadPath"`
}

type ArtifactsConfig map[types.ArtifactType]Location

type RemoteURLs struct {
	BF2Demo       string `yaml:"bf2demo"`
	PRDemo        string `yaml:"prdemo"`
	TrackerViewer string `yaml:"tracker"`
}

type Discord struct {
	ChannelID string     `yaml:"channelID"`
	URLS      RemoteURLs `yaml:"urls"`
}

type Server struct {
	Upload       UploadConfig    `yaml:"upload"`
	Artifacts    ArtifactsConfig `yaml:"types"`
	Discord      Discord         `yaml:"discord,omitempty"`
	RoundTimeout time.Duration   `yaml:"roundTimeout,omitempty"`
}

type Config struct {
	Servers map[string]*Server `yaml:"servers"`
}

func New(filename string) (*Config, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var c Config
	err = yaml.Unmarshal(content, &c)
	if err != nil {
		return nil, err
	}

	return &c, nil
}
