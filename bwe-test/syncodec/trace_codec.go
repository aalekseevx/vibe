// SPDX-FileCopyrightText: 2025 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package syncodec

import (
	"errors"
	"sync"
	"time"

	"github.com/aalekseevx/vibe/bwe-test/traces"
)

// TraceCodec implements a codec that uses pre-recorded traces for different qualities.
// It provides an API to switch between different quality traces.
type TraceCodec struct {
	writer FrameWriter

	// Map of quality identifiers to trace data
	traces map[string]*traces.Trace

	// Current active quality
	currentQuality string

	// Mutex to protect quality switching
	qualityMutex sync.RWMutex

	// Channel to signal done
	done chan struct{}
}

// TraceCodecOption is a function that configures a TraceCodec.
type TraceCodecOption func(*TraceCodec) error

// WithInitialQuality sets the initial quality for the codec.
func WithInitialQuality(quality string) TraceCodecOption {
	return func(tc *TraceCodec) error {
		if _, exists := tc.traces[quality]; !exists {
			return errors.New("specified quality does not exist in the provided traces")
		}
		tc.currentQuality = quality
		return nil
	}
}

// NewTraceCodec creates a new TraceCodec with the specified frame writer and traces.
func NewTraceCodec(writer FrameWriter, traces map[string]*traces.Trace, opts ...TraceCodecOption) (*TraceCodec, error) {
	if len(traces) == 0 {
		return nil, errors.New("no traces provided")
	}

	// Set default quality to the first one in the map
	var defaultQuality string
	for quality := range traces {
		defaultQuality = quality
		break
	}

	tc := &TraceCodec{
		writer:         writer,
		traces:         traces,
		currentQuality: defaultQuality,
		qualityMutex:   sync.RWMutex{},
		done:           make(chan struct{}),
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(tc); err != nil {
			return nil, err
		}
	}

	return tc, nil
}

// GetCurrentQuality returns the current active quality.
func (c *TraceCodec) GetCurrentQuality() string {
	c.qualityMutex.RLock()
	defer c.qualityMutex.RUnlock()
	return c.currentQuality
}

// SetQuality switches to a different quality trace.
func (c *TraceCodec) SetQuality(quality string) error {
	c.qualityMutex.Lock()
	defer c.qualityMutex.Unlock()

	if _, exists := c.traces[quality]; !exists {
		return errors.New("specified quality does not exist in the provided traces")
	}

	c.currentQuality = quality
	return nil
}

// GetAvailableQualities returns a list of available quality identifiers.
func (c *TraceCodec) GetAvailableQualities() []string {
	qualities := make([]string, 0, len(c.traces))
	for quality := range c.traces {
		qualities = append(qualities, quality)
	}
	return qualities
}

// Start begins the codec operation, playing back frames from the current trace.
func (c *TraceCodec) Start() {
	var frameIndex int
	var lastTimestamp float64
	
	// Start with a small initial delay
	time.Sleep(100 * time.Millisecond)
	
	for {
		select {
		case <-c.done:
			return
		default:
			c.qualityMutex.RLock()
			trace := c.traces[c.currentQuality]
			c.qualityMutex.RUnlock()

			if len(trace.Frames) == 0 {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			if frameIndex >= len(trace.Frames) {
				frameIndex = 0 // Loop back to the beginning
				lastTimestamp = 0
			}

			frame := trace.Frames[frameIndex]
			
			// Calculate the duration based on the timestamp difference
			var frameDuration time.Duration
			if frameIndex == 0 || lastTimestamp == 0 {
				frameDuration = 33 * time.Millisecond // Default to ~30fps for the first frame
			} else {
				// Convert timestamp difference to duration
				timestampDiff := frame.Timestamp - lastTimestamp
				frameDuration = time.Duration(timestampDiff * float64(time.Second))
				
				// Ensure a reasonable duration (between 10ms and 100ms)
				if frameDuration < 10*time.Millisecond {
					frameDuration = 10 * time.Millisecond
				} else if frameDuration > 100*time.Millisecond {
					frameDuration = 100 * time.Millisecond
				}
			}
			
			// Sleep for the frame duration
			time.Sleep(frameDuration)
			
			// Send the frame
			c.writer.WriteFrame(Frame{
				Content:  make([]byte, frame.Size),
				Duration: frameDuration,
			})
			
			lastTimestamp = frame.Timestamp
			frameIndex++
		}
	}
}

// Close stops the codec and cleans up resources.
func (c *TraceCodec) Close() error {
	close(c.done)
	return nil
}
