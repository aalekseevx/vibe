// SPDX-FileCopyrightText: 2025 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/pion/transport/v3/vnet"
	"gopkg.in/yaml.v3"
)

// Config represents the YAML configuration for the bandwidth estimation tests.
type Config struct {
	LogLevel   string     `yaml:"log_level"`
	UseSyncTest bool      `yaml:"use_sync_test"`
	TestCases  []TestCase `yaml:"test_cases"`
}

// TestCase defines a single test case configuration.
type TestCase struct {
	Name             string              `yaml:"name"`
	SenderMode       string              `yaml:"sender_mode"`
	FlowMode         string              `yaml:"flow_mode"`
	PathCharacteristic PathCharacteristic `yaml:"path_characteristic"`
}

// PathCharacteristic defines the network characteristics for the test.
type PathCharacteristic struct {
	Phases []PhaseConfig `yaml:"phases"`
}

// PhaseConfig defines a single phase of the network simulation with specific characteristics.
type PhaseConfig struct {
	Duration  time.Duration `yaml:"duration"`
	Capacity  int           `yaml:"capacity"`
	MaxBurst  int           `yaml:"max_burst"`
}

// LoadConfig loads the configuration from a YAML file.
func LoadConfig() (Config, error) {
	configPath := flag.String("config", "", "path to config")
	flag.Parse()
	
	if *configPath == "" {
		return Config{}, fmt.Errorf("config path is required")
	}
	
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

// ParseSenderMode converts a string sender mode to the corresponding enum value.
func ParseSenderMode(mode string) (senderMode, error) {
	switch mode {
	case "simulcast":
		return simulcastSenderMode, nil
	case "abr":
		return abrSenderMode, nil
	default:
		return 0, fmt.Errorf("unknown sender mode: %s", mode)
	}
}

// ParseFlowMode converts a string flow mode to the corresponding enum value.
func ParseFlowMode(mode string) (flowMode, error) {
	switch mode {
	case "single":
		return singleFlowMode, nil
	case "multiple":
		return multipleFlowsMode, nil
	default:
		return 0, fmt.Errorf("unknown flow mode: %s", mode)
	}
}
