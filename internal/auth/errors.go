package auth

import "errors"

// ErrInvalidGrant reports that the auth server rejected the refresh token itself,
// so no retry can succeed and the user has to log in again.
var ErrInvalidGrant = errors.New("refresh token was rejected")
