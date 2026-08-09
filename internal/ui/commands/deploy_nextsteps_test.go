package commands

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNextStepLines(t *testing.T) {
	lines := nextStepLines("my-app", false)
	require.Len(t, lines, 3)

	joined := strings.Join(lines, "\n")
	assert.Contains(t, joined, "cerebrium logs my-app")
	assert.Contains(t, joined, "cerebrium containers list my-app")
	assert.Contains(t, joined, "cerebrium metrics resources my-app")
}

// Descriptions line up because padding is measured on the uncoloured command.
// Padding a string that already carries escape codes would skew every row, so
// the coloured variant has to align identically to the plain one.
func TestNextStepLinesAlignDescriptions(t *testing.T) {
	descriptions := []string{
		"stream runtime logs",
		"see what is running",
		"check CPU, memory and GPU usage",
	}

	for _, colorize := range []bool{false, true} {
		lines := nextStepLines("my-app", colorize)
		require.Len(t, lines, len(descriptions))

		var columns []int
		for i, line := range lines {
			column := strings.Index(stripANSI(line), descriptions[i])
			require.NotEqual(t, -1, column, "description %q missing from %q", descriptions[i], line)
			columns = append(columns, column)
		}

		for _, column := range columns {
			assert.Equal(t, columns[0], column,
				"descriptions should start at the same column (colorize=%v)", colorize)
		}
	}
}

func stripANSI(s string) string {
	var out strings.Builder
	inEscape := false
	for _, r := range s {
		switch {
		case r == '\x1b':
			inEscape = true
		case inEscape && r == 'm':
			inEscape = false
		case !inEscape:
			out.WriteRune(r)
		}
	}
	return out.String()
}
