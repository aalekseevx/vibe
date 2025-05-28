// SPDX-FileCopyrightText: 2025 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

// Package main implements virtual network functionality for bandwidth estimation tests.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"testing/synctest"
	"time"

	"github.com/pion/logging"
)

// senderMode defines the type of sender to use in the test.
type senderMode int

const (
	simulcastSenderMode senderMode = iota
	abrSenderMode
)

// flowMode defines whether to use a single flow or multiple flows in the test.
type flowMode int

const (
	singleFlowMode flowMode = iota
	multipleFlowsMode
)

func main() {
	config, err := LoadConfig()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if config.UseSyncTest {
		synctest.Run(func() {
			simulation(config)
			synctest.Wait()
		})
	} else {
		simulation(config)
	}
}

func simulation(config Config) {
	loggerFactory, err := getLoggerFactory(config.LogLevel)
	if err != nil {
		log.Fatalf("get logger factory: %v", err)
	}

	logger := loggerFactory.NewLogger("bwe_test_runner")
	for _, testCase := range config.TestCases {
		senderMode, err := ParseSenderMode(testCase.Sender.Mode)
		if err != nil {
			logger.Errorf("parse sender mode: %v", err)
			continue
		}

		flowMode, err := ParseFlowMode(testCase.FlowMode)
		if err != nil {
			logger.Errorf("parse flow mode: %v", err)
			continue
		}

		runner := Runner{
			loggerFactory:      loggerFactory,
			logger:             logger,
			name:               testCase.Name,
			senderMode:         senderMode,
			flowMode:           flowMode,
			config:             config,
			testCase:           testCase,
		}
		err = runner.Run()
		if err != nil {
			logger.Errorf("runner: %v", err)
		}
	}
}

var errUnknownLogLevel = errors.New("unknown log level")

func getLoggerFactory(logLevel string) (*logging.DefaultLoggerFactory, error) {
	logLevels := map[string]logging.LogLevel{
		"disable": logging.LogLevelDisabled,
		"error":   logging.LogLevelError,
		"warn":    logging.LogLevelWarn,
		"info":    logging.LogLevelInfo,
		"debug":   logging.LogLevelDebug,
		"trace":   logging.LogLevelTrace,
	}

	level, ok := logLevels[strings.ToLower(logLevel)]
	if !ok {
		return nil, fmt.Errorf("%w: %s", errUnknownLogLevel, logLevel)
	}

	loggerFactory := &logging.DefaultLoggerFactory{
		Writer:          os.Stdout,
		DefaultLogLevel: level,
		ScopeLevels:     make(map[string]logging.LogLevel),
	}

	return loggerFactory, nil
}

// Runner manages the execution of bandwidth estimation tests.
type Runner struct {
	loggerFactory      *logging.DefaultLoggerFactory
	logger             logging.LeveledLogger
	name               string
	senderMode         senderMode
	flowMode           flowMode
	config             Config
	testCase           TestCase
}

var errUnknownFlowMode = errors.New("unknown flow mode")

// Run executes the test based on the configured flow mode.
func (r *Runner) Run() error {
	// Get the path characteristic to use
	pathCharacteristic, err := GetPathCharacteristic(r.config, r.testCase)
	if err != nil {
		return err
	}

	switch r.flowMode {
	case singleFlowMode:
		err := r.runVariableAvailableCapacitySingleFlow(pathCharacteristic)
		if err != nil {
			return fmt.Errorf("run variable available capacity single flow: %w", err)
		}
	case multipleFlowsMode:
		err := r.runVariableAvailableCapacityMultipleFlows(pathCharacteristic)
		if err != nil {
			return fmt.Errorf("run variable available capacity multiple flows: %w", err)
		}
	default:
		return fmt.Errorf("%w: %v", errUnknownFlowMode, r.flowMode)
	}

	return nil
}

func (r *Runner) runVariableAvailableCapacitySingleFlow(pathCharacteristic PathCharacteristic) error {
	nm, err := NewManager()
	if err != nil {
		return fmt.Errorf("new manager: %w", err)
	}

	dataDir := fmt.Sprintf("data/%v", r.name)
	err = os.MkdirAll(dataDir, 0o750)
	if err != nil {
		return fmt.Errorf("mkdir data: %w", err)
	}

	flow, err := NewSimpleFlow(r.loggerFactory, nm, 0, r.senderMode, dataDir, r.config, r.testCase)
	if err != nil {
		return fmt.Errorf("setup simple flow: %w", err)
	}
	defer func(flow Flow) {
		err = flow.Close()
		if err != nil {
			r.logger.Errorf("flow close: %v", err)
		}
	}(flow)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		err = flow.sender.sender.Start(ctx)
		if err != nil {
			r.logger.Errorf("sender start: %v", err)
		}
	}()

	r.runNetworkSimulation(pathCharacteristic.Phases, nm)

	return nil
}

func (r *Runner) runVariableAvailableCapacityMultipleFlows(pathCharacteristic PathCharacteristic) error {
	nm, err := NewManager()
	if err != nil {
		return fmt.Errorf("new manager: %w", err)
	}

	dataDir := fmt.Sprintf("data/%v", r.name)
	err = os.MkdirAll(dataDir, 0o750)
	if err != nil {
		return fmt.Errorf("mkdir data: %w", err)
	}

	for i := 0; i < 2; i++ {
		flow, err := NewSimpleFlow(r.loggerFactory, nm, i, r.senderMode, dataDir, r.config, r.testCase)
		defer func(flow Flow) {
			err = flow.Close()
			if err != nil {
				r.logger.Errorf("flow close: %v", err)
			}
		}(flow)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			err = flow.sender.sender.Start(ctx)
			if err != nil {
				r.logger.Errorf("sender start: %v", err)
			}
		}()
	}

	r.runNetworkSimulation(pathCharacteristic.Phases, nm)

	return nil
}

func (r *Runner) runNetworkSimulation(phases []PhaseConfig, nm *NetworkManager) {
	for _, phase := range phases {
		r.logger.Infof("enter next phase: %v", phase)
		nm.SetCapacity(
			phase.Capacity,
			phase.MaxBurst,
		)
		time.Sleep(phase.Duration)
	}
}
