package config

import (
	"os"

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
	Headers map[string]string `yaml:"header,omitempty"`
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
	Location   string  `yaml:"location"`
	UploadPath string  `yaml:"uploadPath"`
	MovePath   *string `yaml:"movePath,omitempty"`
}

type ArtifactsConfig map[ArtifactType]Location

type Discord struct {
	ChannelID string            `yaml:"channelID"`
	URLS      map[string]string `yaml:"urls"`
}

type Server struct {
	Upload    UploadConfig    `yaml:"upload"`
	Artifacts ArtifactsConfig `yaml:"types"`
	Discord   Discord         `yaml:"discord,omitempty"`
}

type Config struct {
	FailedUploadPath string             `yaml:"failedUploadPath"`
	Servers          map[string]*Server `yaml:"servers"`
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
