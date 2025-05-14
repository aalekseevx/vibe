// SPDX-FileCopyrightText: 2025 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package syncodec

import (
	"errors"
	"sync"
	"time"

	"github.com/aalekseevx/vibe/bwe-test/traces"
)

type Trace struct {
	*traces.Trace
	TrackID   uint32
	QualityID uint32
}

// TraceCodec implements a codec that uses pre-recorded traces for different qualities.
// It provides an API to switch between different quality traces.
type TraceCodec struct {
	writer FrameWriter

	// Map of quality identifiers to trace data
	traces map[string]Trace

	// Current active quality
	currentQuality string

	frameIndex int

	// Mutex to protect quality switching
	qualityMutex sync.RWMutex

	// Channel to signal done
	done chan struct{}
}

// NewTraceCodec creates a new TraceCodec with the specified frame writer and traces.
func NewTraceCodec(writer FrameWriter, traces map[string]Trace, initialQuality string) (*TraceCodec, error) {
	if len(traces) == 0 {
		return nil, errors.New("no traces provided")
	}

	tc := &TraceCodec{
		writer:         writer,
		traces:         traces,
		currentQuality: "",
		frameIndex:     0,
		qualityMutex:   sync.RWMutex{},
		done:           make(chan struct{}),
	}

	err := tc.SetQuality(initialQuality)
	if err != nil {
		return nil, err
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
	c.frameIndex = 0 // Reset frame index to start from the beginning of the new trace
	return nil
}

// Start begins the codec operation, playing back frames from the current trace.
func (c *TraceCodec) Start() {
	for {
		select {
		case <-c.done:
			return
		default:
			c.qualityMutex.RLock()
			trace := c.traces[c.currentQuality]
			c.qualityMutex.RUnlock()

			frame := trace.Frames[c.frameIndex]

			var frameDuration time.Duration
			if c.frameIndex+1 >= len(trace.Frames) {
				prevFrame := trace.Frames[c.frameIndex]
				frameDuration = time.Duration((frame.Timestamp - prevFrame.Timestamp) * float64(time.Second))
			} else {
				nextFrame := trace.Frames[c.frameIndex+1]
				frameDuration = time.Duration((nextFrame.Timestamp - frame.Timestamp) * float64(time.Second))
			}

			// Sleep for the frame duration
			time.Sleep(frameDuration)

			// Send the frame
			c.writer.WriteFrame(Frame{
				Content:   make([]byte, frame.Size),
				Duration:  frameDuration,
				TrackID:   trace.TrackID,
				QualityID: trace.QualityID,
			})

			c.frameIndex = (c.frameIndex + 1) % len(trace.Frames)
		}
	}
}

// Close stops the codec and cleans up resources.
func (c *TraceCodec) Close() error {
	close(c.done)
	return nil
}
