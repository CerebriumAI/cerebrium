package apps

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/sebdah/goldie/v2"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/ui"
)

//go:generate go test -v -run TestFormatAppDetailsTable -update

// The success panel reaches the terminal through tea.Println for scrollback, so
// View() is empty once details load and the harness cannot golden it. Render it
// directly instead — otherwise the panel every `apps get` prints is untested.
func TestFormatAppDetailsTable(t *testing.T) {
	baseTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tcs := []struct {
		name       string
		golden     string
		appDetails *api.AppDetails
	}{
		{
			name:   "GPU app",
			golden: "get_success_gpu",
			appDetails: &api.AppDetails{
				ID:                         "test-app-gpu",
				CreatedAt:                  baseTime,
				UpdatedAt:                  baseTime.Add(time.Hour),
				Hardware:                   "GPU",
				CPU:                        "4",
				Memory:                     "16",
				GPUCount:                   "1",
				CooldownPeriodSeconds:      "60",
				MinReplicaCount:            "0",
				MaxReplicaCount:            "5",
				ResponseGracePeriodSeconds: "900",
				Status:                     "ACTIVE",
				LastBuildStatus:            "SUCCESS",
				LatestBuildID:              "build-123",
			},
		},
		{
			name:   "CPU only app",
			golden: "get_success_cpu",
			appDetails: &api.AppDetails{
				ID:                         "test-app-cpu",
				CreatedAt:                  baseTime,
				UpdatedAt:                  baseTime,
				Hardware:                   "CPU",
				CPU:                        "2",
				Memory:                     "8",
				GPUCount:                   "0",
				CooldownPeriodSeconds:      "30",
				MinReplicaCount:            "1",
				MaxReplicaCount:            "3",
				ResponseGracePeriodSeconds: "600",
				Status:                     "PENDING",
				LastBuildStatus:            "BUILDING",
			},
		},
		{
			name:   "unparseable numeric fields",
			golden: "get_parse_errors",
			appDetails: &api.AppDetails{
				ID:                         "parse-error-app",
				CreatedAt:                  baseTime,
				UpdatedAt:                  baseTime,
				Hardware:                   "GPU",
				CPU:                        "invalid",
				Memory:                     "not-a-number",
				GPUCount:                   "bad",
				CooldownPeriodSeconds:      "60",
				MinReplicaCount:            "0",
				MaxReplicaCount:            "5",
				ResponseGracePeriodSeconds: "900",
				Status:                     "ACTIVE",
				LastBuildStatus:            "SUCCESS",
				LatestBuildID:              "build-123",
			},
		},
		{
			name:   "empty hardware fields",
			golden: "get_empty_fields",
			appDetails: &api.AppDetails{
				ID:                    "empty-app",
				CreatedAt:             baseTime,
				UpdatedAt:             baseTime,
				CooldownPeriodSeconds: "60",
				MinReplicaCount:       "0",
				MaxReplicaCount:       "1",
				Status:                "PENDING",
				LastBuildStatus:       "PENDING",
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// Match the harness, so these goldens stay stable across environments
			lipgloss.SetColorProfile(termenv.Ascii)

			model := NewGetView(t.Context(), GetConfig{
				ProjectID:     "test-project",
				AppID:         tc.appDetails.ID,
				DisplayConfig: ui.DisplayConfig{IsInteractive: true},
			})
			model.appDetails = tc.appDetails
			model.loading = false

			// The harness trims View() output before comparing; do the same so
			// trailing padding from the panel does not decide the result
			rendered := strings.TrimSpace(model.formatAppDetailsTable())

			g := goldie.New(t, goldie.WithFixtureDir("testdata"), goldie.WithNameSuffix(".golden"))
			g.Assert(t, tc.golden, []byte(rendered))
		})
	}
}
