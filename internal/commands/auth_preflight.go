package commands

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cerebriumai/cerebrium/internal/authsession"
	"github.com/cerebriumai/cerebrium/internal/ui"
	"github.com/cerebriumai/cerebrium/pkg/config"
	"github.com/spf13/cobra"
)

// ensureAuthenticated resolves credentials before a command starts work, so an unusable
// session fails in one round trip instead of after packaging files or building an image.
// Interactive sessions are offered the login flow and continue into the command with it.
func ensureAuthenticated(cmd *cobra.Command, cfg *config.Config, displayOpts ui.DisplayConfig) error {
	if !authsession.Required(cmd) {
		return nil
	}

	_, err := authsession.Token(cmd.Context(), cfg)
	if err == nil {
		return nil
	}

	expired := errors.Is(err, authsession.ErrSessionExpired)
	if !expired && !errors.Is(err, authsession.ErrNotLoggedIn) {
		return ui.NewConfigurationError(err)
	}

	if !displayOpts.IsInteractive || !displayOpts.StdinIsTTY {
		return ui.NewConfigurationError(err)
	}

	headline := "You are not logged in."
	if expired {
		headline = "Your session has expired."
	}
	fmt.Println(ui.WarningStyle.Render(headline))
	fmt.Print("Log in now? (Y/n): ")

	if !readLoginConsent(os.Stdin) {
		return ui.NewConfigurationError(err)
	}

	return runInteractiveLogin(cmd.Context(), cfg, displayOpts)
}

// readLoginConsent reads a single line from r and interprets it as a confirmation
// response. An empty line defaults to yes (the prompt is Y/n); EOF or any read
// error is a decline so a closed stdin can never consent
func readLoginConsent(r io.Reader) bool {
	line, err := bufio.NewReader(r).ReadString('\n')
	response := strings.ToLower(strings.TrimSpace(line))
	if err != nil && response == "" {
		return false
	}
	return response == "" || response == "y" || response == "yes"
}
