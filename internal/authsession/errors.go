package authsession

import (
	"errors"

	"github.com/cerebriumai/cerebrium/pkg/config"
)

var (
	// ErrNotLoggedIn reports that no credentials are stored at all.
	ErrNotLoggedIn = errors.New("not logged in. Please run 'cerebrium login', or set " + config.ServiceAccountEnvVar + " for non-interactive use")

	// ErrSessionExpired reports that stored credentials exist but can no longer be renewed.
	ErrSessionExpired = errors.New("your session has expired. Please run 'cerebrium login' to continue")
)
