package authsession

import "github.com/spf13/cobra"

// annotationSkipAuth marks a command tree that works without Cerebrium credentials
const annotationSkipAuth = "cerebrium.ai/skip-auth"

// WithoutAuth marks cmd, and everything under it, as usable without credentials.
func WithoutAuth(cmd *cobra.Command) *cobra.Command {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[annotationSkipAuth] = "true"
	return cmd
}

// Required reports whether cmd needs credentials before it can do anything useful.
func Required(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Annotations[annotationSkipAuth] == "true" {
			return false
		}
	}
	return true
}
