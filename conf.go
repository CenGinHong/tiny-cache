package main

import (
	"gopkg.in/yaml.v2"
	"io"
	"os"
)

type Config struct {
	ApiPort  string   `yaml:"apiPort"`
	PeerPort string   `yaml:"peerPort"`
	Peer     []string `yaml:"peer"`
}

func LoadConfig(path string) (config *Config, err error) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	yamlStr, err := io.ReadAll(file)
	if err != nil {
		return
	}

	config = &Config{}
	if err = yaml.Unmarshal(yamlStr, config); err != nil {
		return nil, err
	}
	return
}
