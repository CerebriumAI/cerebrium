package api

import (
	"fmt"
	"strings"
)

// NormalizeAppID ensures an app ID carries its project ID prefix, so commands can
// accept either the bare app name or the fully qualified ID.
func NormalizeAppID(projectID, appName string) string {
	expectedPrefix := projectID + "-"
	if strings.HasPrefix(appName, expectedPrefix) {
		return appName
	}
	return fmt.Sprintf("%s-%s", projectID, appName)
}
