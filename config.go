package main

import (
	"os"

	"github.com/goccy/go-yaml"
)

type SCPCofig struct {
}

type HTTPSConfig struct {
}

type SFTPConfig struct {
}

type UploadConfig struct {
	SCP   *SCPCofig
	HTTPS *HTTPSConfig
	SFTP  *SFTPConfig
}

type Location struct {
	Path   string
	SubDir string
}

type Server struct {
	Upload    UploadConfig `yaml:"upload"`
	BF2Demos  Location
	PRDemos   Location
	Summaries Location
}

type Config struct {
	Servers []Server `yaml:"servers"`
}

func newConfig(filename string) (*Config, error) {
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
