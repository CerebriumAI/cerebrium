package authsession

import (
	"errors"
	"fmt"

	"github.com/cerebriumai/cerebrium/pkg/config"
)

var (
	// ErrNotLoggedIn reports that no credentials are stored at all.
	ErrNotLoggedIn = fmt.Errorf("not logged in. Please run 'cerebrium login', or set %s for non-interactive use", config.ServiceAccountEnvVar)

	// ErrSessionExpired reports that stored credentials exist but can no longer be renewed.
	ErrSessionExpired = errors.New("your session has expired. Please run 'cerebrium login' to continue")
)
