// SPDX-FileCopyrightText: 2025 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/aalekseevx/vibe/bwe-test/sender"
	"gopkg.in/yaml.v3"
)

// Config represents the YAML configuration for the bandwidth estimation tests.
type Config struct {
	LogLevel                  string                        `yaml:"log_level"`
	UseSyncTest               bool                          `yaml:"use_sync_test"`
	TracesDir                 string                        `yaml:"traces_dir"`
	SimulcastConfigsPresets   map[string]SimulcastConfig    `yaml:"simulcast_configs_presets"`
	PathCharacteristicPresets map[string]PathCharacteristic `yaml:"path_characteristic_presets"`
	TestCases                 []TestCase                    `yaml:"test_cases"`
}

// TestCase defines a single test case configuration.
type TestCase struct {
	Name                     string       `yaml:"name"`
	FlowMode                 string       `yaml:"flow_mode"`
	PathCharacteristicPreset string       `yaml:"path_characteristic_preset"`
	Sender                   SenderConfig `yaml:"sender"`
}

// SenderConfig defines the configuration for the sender.
type SenderConfig struct {
	Mode             string   `yaml:"mode"`
	SimulcastPresets []string `yaml:"simulcast_presets,omitempty"`
}

// SimulcastConfig defines the configuration for simulcast mode.
type SimulcastConfig struct {
	InitialQuality string                 `yaml:"initial_quality"`
	Qualities      []sender.QualityConfig `yaml:"qualities"`
}

// PathCharacteristic defines the network characteristics for the test.
type PathCharacteristic struct {
	Phases []PhaseConfig `yaml:"phases"`
}

// PhaseConfig defines a single phase of the network simulation with specific characteristics.
type PhaseConfig struct {
	Duration time.Duration `yaml:"duration"`
	Capacity int           `yaml:"capacity"`
	MaxBurst int           `yaml:"max_burst"`
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

// GetPathCharacteristic returns the path characteristic for a test case.
func GetPathCharacteristic(config Config, testCase TestCase) (PathCharacteristic, error) {
	preset, ok := config.PathCharacteristicPresets[testCase.PathCharacteristicPreset]
	if !ok {
		return PathCharacteristic{}, fmt.Errorf("path characteristic preset not found: %s", testCase.PathCharacteristicPreset)
	}
	return preset, nil
}

// GetSimulcastConfigs returns the simulcast configs for a test case.
func GetSimulcastConfigs(config Config, testCase TestCase) ([]SimulcastConfig, error) {
	if testCase.Sender.Mode != "simulcast" {
		return nil, nil
	}

	presets := make([]SimulcastConfig, len(testCase.Sender.SimulcastPresets))
	for i, presetName := range testCase.Sender.SimulcastPresets {
		preset, ok := config.SimulcastConfigsPresets[presetName]
		if !ok {
			return nil, fmt.Errorf("simulcast preset not found: %s", presetName)
		}
		presets[i] = preset
	}

	return presets, nil
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
