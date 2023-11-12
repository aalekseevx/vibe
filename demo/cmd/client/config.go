package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Endpoint        string        `yaml:"endpoint"`
	IceServer       string        `yaml:"ice_server"`
	ReportFile      string        `yaml:"report_file"`
	SessionDuration time.Duration `yaml:"session_duration"`
	ReportInterval  time.Duration `yaml:"report_interval"`
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
