// SPDX-FileCopyrightText: 2025 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package sender

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"sync"

	"github.com/pion/logging"
	"github.com/pion/webrtc/v4/pkg/media"

	"github.com/aalekseevx/vibe/bwe-test/syncodec"
	"github.com/aalekseevx/vibe/bwe-test/traces"
)

// TraceCodecSource is a source that uses trace files for different qualities.
type TraceCodecSource struct {
	codec        *syncodec.TraceCodec
	sampleWriter func(media.Sample, []uint32) error
	newFrame     chan syncodec.Frame
	done         chan struct{}
	wg           sync.WaitGroup
	log          logging.LeveledLogger
	qualities    []QualityConfig
}

// QualityConfig defines the configuration for a single quality level.
type QualityConfig struct {
	Name      string `yaml:"name"`
	Bitrate   int    `yaml:"bitrate"`
	TraceFile string `yaml:"trace_file"`
	ID        uint32 `yaml:"id"`
}

// TraceCodecSourceOption is a function that configures a TraceCodecSource.
type TraceCodecSourceOption func(*TraceCodecSource) error

// NewTraceCodecSource creates a new TraceCodecSource with the specified options.
func NewTraceCodecSource(tracesDir string, trackID uint32, qualities []QualityConfig, initialQuality string) (*TraceCodecSource, error) {
	source := &TraceCodecSource{
		sampleWriter: func(_ media.Sample, _ []uint32) error {
			return errors.New("write on uninitialized TraceCodecSource.WriteSample")
		},
		newFrame:  make(chan syncodec.Frame),
		done:      make(chan struct{}),
		wg:        sync.WaitGroup{},
		log:       logging.NewDefaultLoggerFactory().NewLogger("trace_codec_source"),
		qualities: qualities,
	}

	// Ensure we have qualities defined
	if len(qualities) == 0 {
		return nil, errors.New("no qualities defined for trace codec source")
	}

	// Load trace files
	traceFiles, err := loadTraceFiles(tracesDir, trackID, qualities)
	if err != nil {
		return nil, err
	}

	// Create the codec
	codec, err := syncodec.NewTraceCodec(source, traceFiles, initialQuality)
	if err != nil {
		return nil, err
	}
	source.codec = codec

	return source, nil
}

// loadTraceFiles loads the specified trace files from the directory.
func loadTraceFiles(tracesDir string, trackID uint32, qualities []QualityConfig) (map[string]syncodec.Trace, error) {
	traceFiles := make(map[string]syncodec.Trace)

	// Process each specified quality
	for _, quality := range qualities {
		path := filepath.Join(tracesDir, quality.TraceFile)

		// Load the trace file
		trace, err := traces.ReadTraceFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to load trace file %s: %w", path, err)
		}

		traceFiles[quality.Name] = syncodec.Trace{
			Trace:     trace,
			TrackID:   trackID,
			QualityID: quality.ID,
		}
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

type Quality struct {
	Name    string
	Bitrate int
	Active  bool
}

// GetQualities returns the available qualities for the codec.
func (s *TraceCodecSource) GetQualities() []Quality {
	qualities := make([]Quality, len(s.qualities))
	for i, q := range s.qualities {
		qualities[i] = Quality{
			Name:    q.Name,
			Bitrate: q.Bitrate,
			Active:  q.Name == s.codec.GetCurrentQuality(),
		}
	}
	// Sort the qualities by bitrate
	sort.Slice(qualities, func(i, j int) bool {
		return qualities[i].Bitrate < qualities[j].Bitrate
	})
	return qualities
}

// SetWriter sets the sample writer function.
func (s *TraceCodecSource) SetWriter(f func(sample media.Sample, csrc []uint32) error) {
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
			err := s.sampleWriter(media.Sample{Data: frame.Content, Duration: frame.Duration}, []uint32{frame.TrackID, frame.QualityID})
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

func (s *TraceCodecSource) SetTargetBitrate(i int) {
	// Does nothing for now
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
