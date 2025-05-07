// SPDX-FileCopyrightText: 2025 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package sender

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/pion/logging"
	"github.com/pion/webrtc/v4/pkg/media"

	"github.com/aalekseevx/vibe/bwe-test/syncodec"
	"github.com/aalekseevx/vibe/bwe-test/traces"
)

// TraceCodecSource is a source that uses trace files for different qualities.
type TraceCodecSource struct {
	codec          *syncodec.TraceCodec
	sampleWriter   func(media.Sample) error
	newFrame       chan syncodec.Frame
	done           chan struct{}
	wg             sync.WaitGroup
	log            logging.LeveledLogger
	initialQuality string
	tracesDir      string
	qualities      []QualityConfig
}

// QualityConfig defines the configuration for a single quality level.
type QualityConfig struct {
	Name      string `yaml:"name"`
	Bitrate   int    `yaml:"bitrate"`
	TraceFile string `yaml:"trace_file"`
}

// TraceCodecSourceOption is a function that configures a TraceCodecSource.
type TraceCodecSourceOption func(*TraceCodecSource) error

// WithInitialQuality sets the initial quality for the codec.
func WithInitialQuality(quality string) TraceCodecSourceOption {
	return func(s *TraceCodecSource) error {
		s.initialQuality = quality
		return nil
	}
}

// WithTracesDir sets the directory containing trace files.
func WithTracesDir(tracesDir string) TraceCodecSourceOption {
	return func(s *TraceCodecSource) error {
		s.tracesDir = tracesDir
		return nil
	}
}

// WithQualities sets the quality configurations.
func WithQualities(qualities []QualityConfig) TraceCodecSourceOption {
	return func(s *TraceCodecSource) error {
		s.qualities = qualities
		return nil
	}
}

// NewTraceCodecSource creates a new TraceCodecSource with the specified options.
func NewTraceCodecSource(opts ...TraceCodecSourceOption) (*TraceCodecSource, error) {
	source := &TraceCodecSource{
		sampleWriter: func(_ media.Sample) error {
			return errors.New("write on uninitialized TraceCodecSource.WriteSample")
		},
		newFrame: make(chan syncodec.Frame),
		done:     make(chan struct{}),
		wg:       sync.WaitGroup{},
		log:      logging.NewDefaultLoggerFactory().NewLogger("trace_codec_source"),
		tracesDir: "bwe-test/traces/chat_firefox_h264", // Default traces directory
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(source); err != nil {
			return nil, err
		}
	}

	// Ensure we have qualities defined
	if len(source.qualities) == 0 {
		return nil, errors.New("no qualities defined for trace codec source")
	}

	// Load trace files
	traceFiles, err := loadTraceFiles(source.tracesDir, source.qualities)
	if err != nil {
		return nil, err
	}

	// Create codec options from source options
	var codecOpts []syncodec.TraceCodecOption
	
	// Add initial quality option if set
	if source.initialQuality != "" {
		codecOpts = append(codecOpts, syncodec.WithInitialQuality(source.initialQuality))
	}

	// Create the codec
	codec, err := syncodec.NewTraceCodec(source, traceFiles, codecOpts...)
	if err != nil {
		return nil, err
	}
	source.codec = codec

	return source, nil
}

// loadTraceFiles loads the specified trace files from the directory.
func loadTraceFiles(tracesDir string, qualities []QualityConfig) (map[string]*traces.Trace, error) {
	traceFiles := make(map[string]*traces.Trace)
	
	// Process each specified quality
	for _, quality := range qualities {
		path := filepath.Join(tracesDir, quality.TraceFile)
		
		// Load the trace file
		trace, err := traces.ReadTraceFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to load trace file %s: %w", path, err)
		}
		
		traceFiles[quality.Name] = trace
	}
	
	if len(traceFiles) == 0 {
		return nil, fmt.Errorf("no trace files loaded")
	}
	
	return traceFiles, nil
}

// SetQuality sets the quality for the codec.
func (s *TraceCodecSource) SetQuality(quality string) error {
	return s.codec.SetQuality(quality)
}

// GetCurrentQuality returns the current quality.
func (s *TraceCodecSource) GetCurrentQuality() string {
	return s.codec.GetCurrentQuality()
}

// GetAvailableQualities returns a list of available qualities.
func (s *TraceCodecSource) GetAvailableQualities() []string {
	return s.codec.GetAvailableQualities()
}

// SetWriter sets the sample writer function.
func (s *TraceCodecSource) SetWriter(f func(sample media.Sample) error) {
	s.sampleWriter = f
}

// Start begins the encoding process and runs until context is done.
func (s *TraceCodecSource) Start(ctx context.Context) error {
	s.wg.Add(1)
	defer s.wg.Done()
	
	// Start the codec in a goroutine
	go s.codec.Start()
	defer func() {
		if err := s.codec.Close(); err != nil {
			s.log.Errorf("Failed to close codec: %v", err)
		}
	}()
	
	for {
		select {
		case frame := <-s.newFrame:
			err := s.sampleWriter(media.Sample{Data: frame.Content, Duration: frame.Duration})
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return nil
		case <-s.done:
			return nil
		}
	}
}

// WriteFrame writes a frame to the encoder.
func (s *TraceCodecSource) WriteFrame(frame syncodec.Frame) {
	s.newFrame <- frame
}

// Close stops the encoder and cleans up resources.
func (s *TraceCodecSource) Close() error {
	defer s.wg.Wait()
	close(s.done)
	
	return nil
}
