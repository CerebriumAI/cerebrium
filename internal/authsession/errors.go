package authsession

import "errors"

var (
	// ErrNotLoggedIn reports that no credentials are stored at all.
	ErrNotLoggedIn = errors.New("not logged in. Please run 'cerebrium login', or set CEREBRIUM_SERVICE_ACCOUNT_TOKEN for non-interactive use")

	// ErrSessionExpired reports that stored credentials exist but can no longer be renewed.
	ErrSessionExpired = errors.New("your session has expired. Please run 'cerebrium login' to continue")
)
