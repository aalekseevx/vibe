// Package traces provides functionality for reading video trace files.
package traces

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// FrameType represents the type of video frame.
type FrameType rune

const (
	// IFrame represents an I-frame in the video trace.
	IFrame FrameType = 'I'
	// PFrame represents a P-frame in the video trace.
	PFrame FrameType = 'P'
	// BFrame represents a B-frame in the video trace.
	BFrame FrameType = 'B'
	// UFRame represents an unknown frame type.
	UFRame FrameType = 'U'
)

// String returns the string representation of the frame type.
func (ft FrameType) String() string {
	return string(ft)
}

// Frame represents a single frame entry from the trace file.
type Frame struct {
	// Number is the frame number in the sequence.
	Number int
	// Type is the type of the frame (I, P, or B).
	Type FrameType
	// Timestamp is the timestamp of the frame in seconds.
	Timestamp float64
	// Size is the size of the frame in bytes.
	Size int
}

// Trace represents a collection of frames loaded from a trace file.
type Trace struct {
	// Frames is the ordered list of frames in the trace.
	Frames []Frame
}

// ParseFrameType converts a string to a FrameType.
func ParseFrameType(s string) (FrameType, error) {
	switch s {
	case "I":
		return IFrame, nil
	case "P":
		return PFrame, nil
	case "B":
		return BFrame, nil
	case "U":
		return UFRame, nil
	default:
		return 0, fmt.Errorf("invalid frame type: %s", s)
	}
}

// ReadTraceFile reads a trace file and returns a Trace object.
func ReadTraceFile(filename string) (*Trace, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open trace file: %w", err)
	}
	defer file.Close()

	trace := &Trace{
		Frames: make([]Frame, 0),
	}

	scanner := bufio.NewScanner(file)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()

		// Remove comments
		if idx := strings.IndexAny(line, "#%"); idx >= 0 {
			line = line[:idx]
		}

		// Skip empty lines after removing comments
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse the line
		frame, err := parseFrameLine(line)
		if err != nil {
			return nil, fmt.Errorf("error at line %d: %w", lineNumber, err)
		}

		trace.Frames = append(trace.Frames, frame)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading trace file: %w", err)
	}

	return trace, nil
}

func parseFrameLine(line string) (Frame, error) {
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return Frame{}, fmt.Errorf("expected at least 5 fields, got %d", len(fields))
	}

	// Parse frame number
	frameNumber, err := strconv.Atoi(fields[0])
	if err != nil {
		return Frame{}, fmt.Errorf("invalid frame number: %w", err)
	}

	// Parse frame type
	frameType, err := ParseFrameType(fields[1])
	if err != nil {
		return Frame{}, err
	}

	// Parse Timestamp
	ts, err := strconv.ParseFloat(fields[3], 64)
	if err != nil {
		return Frame{}, fmt.Errorf("invalid ts value: %w", err)
	}

	// Parse frame size
	size, err := strconv.Atoi(fields[4])
	if err != nil {
		return Frame{}, fmt.Errorf("invalid frame size: %w", err)
	}

	return Frame{
		Number:    frameNumber,
		Type:      frameType,
		Timestamp: ts,
		Size:      size,
	}, nil
}
