package api

import (
	"testing"
)

func TestGetAuthErr(t *testing.T) {
	tests := []struct {
		name string
		code int
		body string
		want string
	}{
		{
			name: "server message is shown on its own",
			code: 403,
			body: `{"code":"verification_required","message":"Please verify your account."}`,
			want: "Please verify your account.",
		},
		{
			name: "an expired session points at login",
			code: 403,
			body: `{"code":"unauthenticated","message":"Your session has expired or your token is invalid. Please log in again."}`,
			want: "Your session has expired or your token is invalid. Please log in again. Run 'cerebrium login'",
		},
		{
			name: "no server message keeps the generic text",
			code: 403,
			body: ``,
			want: "failed with status code 403. You do not have permission to access this resource",
		},
		{
			name: "401 still asks to log in",
			code: 401,
			body: `{"message":"Unauthorized"}`,
			want: "failed with status code 401. You must log in to use this functionality. Please run 'cerebrium login'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getAuthErr(tt.code, []byte(tt.body)).Error(); got != tt.want {
				t.Errorf("getAuthErr() = %q, want %q", got, tt.want)
			}
		})
	}
}
