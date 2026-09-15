package commands

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cerebriumai/cerebrium/internal/authsession"
	"github.com/cerebriumai/cerebrium/internal/ui"
	"github.com/cerebriumai/cerebrium/pkg/config"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequiresAuth(t *testing.T) {
	tcs := []struct {
		args     []string
		expected bool
	}{
		{args: []string{"login"}, expected: false},
		{args: []string{"init"}, expected: false},
		{args: []string{"version"}, expected: false},
		{args: []string{"config", "get"}, expected: false},
		{args: []string{"region", "set"}, expected: false},
		{args: []string{"deploy"}, expected: true},
		{args: []string{"run"}, expected: true},
		{args: []string{"apps", "list"}, expected: true},
		{args: []string{"projects", "list"}, expected: true},
		{args: []string{"projects", "current"}, expected: false},
		{args: []string{"projects", "set"}, expected: false},
		{args: []string{"ls"}, expected: true},
	}

	rootCmd := NewRootCmd()

	for _, tc := range tcs {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			cmd, _, err := rootCmd.Find(tc.args)
			require.NoError(t, err)

			assert.Equal(t, tc.expected, authsession.Required(cmd))
		})
	}
}

func TestReadLoginConsent(t *testing.T) {
	tcs := []struct {
		name     string
		input    string
		expected bool
	}{
		{name: "empty line defaults to yes", input: "\n", expected: true},
		{name: "y", input: "y\n", expected: true},
		{name: "uppercase yes", input: "YES\n", expected: true},
		{name: "padded y", input: "  y  \n", expected: true},
		{name: "n", input: "n\n", expected: false},
		{name: "anything else", input: "maybe\n", expected: false},
		{name: "closed stdin never consents", input: "", expected: false},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, readLoginConsent(strings.NewReader(tc.input)))
		})
	}
}

func TestEnsureAuthenticated(t *testing.T) {
	t.Run("reports missing credentials as an auth error", func(t *testing.T) {
		t.Setenv("CEREBRIUM_SERVICE_ACCOUNT_TOKEN", "")

		cmd := &cobra.Command{Use: "deploy"}
		cmd.SetContext(t.Context())

		// Not a TTY, so the login prompt is skipped and the error comes straight back
		err := ensureAuthenticated(cmd, &config.Config{}, ui.DisplayConfig{})

		var uiErr *ui.UIError
		require.ErrorAs(t, err, &uiErr)
		assert.Equal(t, ui.ErrorTypeAuth, uiErr.Type)
		assert.True(t, uiErr.SuppressUsage)
		assert.ErrorIs(t, err, authsession.ErrNotLoggedIn)
	})

	t.Run("skips commands that need no credentials", func(t *testing.T) {
		cmd := authsession.WithoutAuth(&cobra.Command{Use: "login"})
		cmd.SetContext(t.Context())

		require.NoError(t, ensureAuthenticated(cmd, &config.Config{}, ui.DisplayConfig{}))
	})
}

func TestSuppressesUsage(t *testing.T) {
	tcs := []struct {
		name     string
		err      error
		expected bool
	}{
		{name: "no error", err: nil, expected: false},
		{name: "plain error still shows usage", err: errors.New("boom"), expected: false},
		{name: "ui error", err: ui.NewAuthError(errors.New("boom")), expected: true},
		{name: "wrapped ui error", err: fmt.Errorf("context: %w", ui.NewAuthError(errors.New("boom"))), expected: true},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, suppressesUsage(tc.err))
		})
	}
}
