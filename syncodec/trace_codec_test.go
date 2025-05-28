// SPDX-FileCopyrightText: 2025 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package syncodec

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/aalekseevx/vibe/bwe-test/traces"
)

// mockFrameWriter is a simple implementation of FrameWriter for testing.
type mockFrameWriter struct {
	frames []Frame
	mu     sync.Mutex
}

func newMockFrameWriter() *mockFrameWriter {
	return &mockFrameWriter{
		frames: make([]Frame, 0),
	}
}

func (m *mockFrameWriter) WriteFrame(frame Frame) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.frames = append(m.frames, frame)
}

func (m *mockFrameWriter) GetFrames() []Frame {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.frames
}

// createMockTrace creates a mock trace with the specified number of frames.
func createMockTrace(frameCount int, frameSize int) *traces.Trace {
	frames := make([]traces.Frame, frameCount)
	for i := 0; i < frameCount; i++ {
		frames[i] = traces.Frame{
			Number:    i,
			Type:      traces.IFrame,
			Timestamp: float64(i) / 30.0, // Assuming 30 FPS
			Size:      frameSize,
		}
	}
	return &traces.Trace{Frames: frames}
}

func TestTraceCodec_Creation(t *testing.T) {
	writer := newMockFrameWriter()

	// Create traces for different qualities
	traces := map[string]*traces.Trace{
		"low":    createMockTrace(10, 1000),
		"medium": createMockTrace(10, 2000),
		"high":   createMockTrace(10, 3000),
	}

	// Test with specific initial quality
	codec, err := NewTraceCodec(writer, traces, "medium")
	assert.NoError(t, err)
	assert.Equal(t, "medium", codec.GetCurrentQuality())

	// Test with invalid initial quality
	_, err = NewTraceCodec(writer, traces, "invalid")
	assert.Error(t, err)
}

func TestTraceCodec_QualitySwitching(t *testing.T) {
	writer := newMockFrameWriter()

	// Create traces for different qualities
	traces := map[string]*traces.Trace{
		"low":    createMockTrace(10, 1000),
		"medium": createMockTrace(10, 2000),
		"high":   createMockTrace(10, 3000),
	}

	codec, err := NewTraceCodec(writer, traces, "low")
	assert.NoError(t, err)

	// Test quality switching
	assert.Equal(t, "low", codec.GetCurrentQuality())

	err = codec.SetQuality("medium")
	assert.NoError(t, err)
	assert.Equal(t, "medium", codec.GetCurrentQuality())

	err = codec.SetQuality("high")
	assert.NoError(t, err)
	assert.Equal(t, "high", codec.GetCurrentQuality())

	// Test switching to invalid quality
	err = codec.SetQuality("invalid")
	assert.Error(t, err)
	assert.Equal(t, "high", codec.GetCurrentQuality()) // Should remain unchanged

	// Test getting available qualities
	qualities := codec.GetAvailableQualities()
	assert.Len(t, qualities, 3)
	assert.Contains(t, qualities, "low")
	assert.Contains(t, qualities, "medium")
	assert.Contains(t, qualities, "high")
}

func TestTraceCodec_FrameGeneration(t *testing.T) {
	writer := newMockFrameWriter()

	// Create traces for different qualities
	traces := map[string]*traces.Trace{
		"low":  createMockTrace(5, 1000),
		"high": createMockTrace(5, 3000),
	}

	codec, err := NewTraceCodec(writer, traces, "low")
	assert.NoError(t, err)

	// Start the codec in a goroutine
	go func() {
		codec.Start()
	}()

	// Wait for a few frames to be generated
	time.Sleep(200 * time.Millisecond)

	// Switch quality
	err = codec.SetQuality("high")
	assert.NoError(t, err)

	// Wait for more frames to be generated
	time.Sleep(200 * time.Millisecond)

	// Stop the codec
	err = codec.Close()
	assert.NoError(t, err)

	// Check that frames were generated
	frames := writer.GetFrames()
	assert.NotEmpty(t, frames)

	// Verify that some frames have the expected size for "low" quality
	lowQualityFound := false
	highQualityFound := false

	for _, frame := range frames {
		if len(frame.Content) == 1000 {
			lowQualityFound = true
		}
		if len(frame.Content) == 3000 {
			highQualityFound = true
		}
	}

	// We should have frames from both qualities
	assert.True(t, lowQualityFound, "No frames found with low quality size")
	assert.True(t, highQualityFound, "No frames found with high quality size")
}
