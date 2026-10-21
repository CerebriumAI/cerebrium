package ui

import (
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseOutputFormat(t *testing.T) {
	tcs := []struct {
		name    string
		args    []string
		want    string
		wantErr string
	}{
		{name: "defaults to table", args: nil, want: OutputTable},
		{name: "long flag", args: []string{"--output", "json"}, want: OutputJSON},
		{name: "short flag", args: []string{"-o", "json"}, want: OutputJSON},
		{name: "explicit table", args: []string{"-o", "table"}, want: OutputTable},
		{name: "unsupported format", args: []string{"-o", "yaml"}, wantErr: "invalid output format: yaml"},
		{name: "empty format", args: []string{"-o", ""}, wantErr: "invalid output format"},
		{name: "casing is not coerced", args: []string{"-o", "JSON"}, wantErr: "invalid output format: JSON"},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "test", RunE: func(*cobra.Command, []string) error { return nil }}
			AddOutputFlag(cmd)
			cmd.SetArgs(tc.args)
			require.NoError(t, cmd.Execute())

			got, err := ParseOutputFormat(cmd)

			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				// The message has to name the accepted values, or the caller is stuck guessing
				assert.Contains(t, err.Error(), "table, json")
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// A command that never registered the flag should surface that as an error rather
// than silently reporting table.
func TestParseOutputFormatWithoutFlagRegistered(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}

	_, err := ParseOutputFormat(cmd)

	assert.Error(t, err)
}

func TestPrintJSON(t *testing.T) {
	type payload struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	out := captureStdout(t, func() {
		require.NoError(t, PrintJSON([]payload{{Name: "a", Count: 1}}))
	})

	assert.JSONEq(t, `[{"name":"a","count":1}]`, out)
	// Indented, so a human reading raw output can follow it
	assert.Contains(t, out, "\n  {")
	assert.True(t, len(out) > 0 && out[len(out)-1] == '\n', "should end with a newline")
}

// An empty slice must not print as null — consumers iterate the result directly.
func TestPrintJSONEmptySlice(t *testing.T) {
	out := captureStdout(t, func() {
		require.NoError(t, PrintJSON([]string{}))
	})

	assert.Equal(t, "[]\n", out)
}

func TestPrintJSONUnmarshalableValue(t *testing.T) {
	out := captureStdout(t, func() {
		assert.Error(t, PrintJSON(make(chan int)))
	})

	assert.Empty(t, out, "nothing should reach stdout when encoding fails")
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = original })

	fn()

	require.NoError(t, w.Close())
	captured, err := io.ReadAll(r)
	require.NoError(t, err)

	return string(captured)
}

// Guards against PrintJSON drifting to a non-deterministic encoder; agents diff
// this output between runs.
func TestPrintJSONIsStable(t *testing.T) {
	value := map[string]int{"b": 2, "a": 1}

	first := captureStdout(t, func() { require.NoError(t, PrintJSON(value)) })
	second := captureStdout(t, func() { require.NoError(t, PrintJSON(value)) })

	assert.Equal(t, first, second)

	var decoded map[string]int
	require.NoError(t, json.Unmarshal([]byte(first), &decoded))
	assert.Equal(t, value, decoded)
}
