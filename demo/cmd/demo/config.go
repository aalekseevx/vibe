package main

import (
	"flag"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Port       int      `yaml:"port"`
	VideoPaths []string `yaml:"video_paths"`
	IceServer  string   `yaml:"ice_server"`
}

func LoadConfig() (Config, error) {
	configPath := flag.String("config", "", "path to config")
	flag.Parse()
	configBytes, err := os.ReadFile(*configPath)
	if err != nil {
		return Config{}, fmt.Errorf("read file: %w", err)
	}
	var config Config
	err = yaml.Unmarshal(configBytes, &config)
	if err != nil {
		return Config{}, fmt.Errorf("yaml unmarshal: %w", err)
	}

	return config, nil
}
