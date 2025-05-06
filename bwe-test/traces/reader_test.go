package traces

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFrameType(t *testing.T) {
	tests := []struct {
		input    string
		expected FrameType
		wantErr  bool
	}{
		{"I", IFrame, false},
		{"P", PFrame, false},
		{"B", BFrame, false},
		{"X", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseFrameType(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, got)
			}
		})
	}
}

func TestReadTraceFile(t *testing.T) {
	// Create a temporary test file
	content := `% Nframe | type | X | timestamp | size (Bytes)
% Fields labeled as X are left for backwards compatibility
0 U 0. 0.000000 1730
1 U 0. 0.034754 223 % This is comment
2 U 0. 0.069687 156
3 U 0. 0.103672 181
4 U 0. 0.131775 102
`
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test_trace.txt")
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err, "Failed to create temp file")

	// Test reading the file
	trace, err := ReadTraceFile(tmpFile)
	require.NoError(t, err, "ReadTraceFile() should not return an error")

	// Verify the results
	expectedFrames := []Frame{
		{Number: 0, Type: UFRame, Timestamp: 0, Size: 1730},
		{Number: 1, Type: UFRame, Timestamp: 0.034754, Size: 223},
		{Number: 2, Type: UFRame, Timestamp: 0.069687, Size: 156},
		{Number: 3, Type: UFRame, Timestamp: 0.103672, Size: 181},
		{Number: 4, Type: UFRame, Timestamp: 0.131775, Size: 102},
	}

	assert.Equal(t, expectedFrames, trace.Frames)
}
