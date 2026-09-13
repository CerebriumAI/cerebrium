package metrics

import (
	"testing"
	"time"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sample(value float64) api.MetricValue {
	return api.MetricValue{Value: value, Valid: true}
}

func TestSummarise(t *testing.T) {
	data := api.ChartData{
		Timestamps: []int64{1, 2, 3},
		Series: []api.ChartSeries{
			{Name: "Max", Data: []api.MetricValue{sample(1.5), {}, sample(3.25)}},
			{Name: "P50", Data: []api.MetricValue{{}, {}, {}}},
		},
	}

	got := summarise(data, "cores")

	assert.Equal(t, "cores", got.Unit)
	require.Len(t, got.Series, 2)

	assert.Equal(t, "Max", got.Series[0].Name)
	require.NotNil(t, got.Series[0].Peak)
	assert.Equal(t, 3.25, *got.Series[0].Peak)

	// A series of nothing but padding has no peak — not a peak of zero.
	assert.Equal(t, "P50", got.Series[1].Name)
	assert.Nil(t, got.Series[1].Peak)
}

func TestSummariseKeepsRealZero(t *testing.T) {
	data := api.ChartData{
		Series: []api.ChartSeries{{Name: "Max", Data: []api.MetricValue{sample(0)}}},
	}

	got := summarise(data, "GB")

	require.NotNil(t, got.Series[0].Peak)
	assert.Equal(t, float64(0), *got.Series[0].Peak)
}

func TestFormatPeak(t *testing.T) {
	peak := 2.5
	metric := metricSummary{Series: []seriesPeak{
		{Name: "Max", Peak: &peak},
		{Name: "P50", Peak: nil},
	}}

	assert.Equal(t, "2.50", formatPeak(metric, 0))
	assert.Equal(t, "-", formatPeak(metric, 1))
	assert.Equal(t, "-", formatPeak(metric, 2), "a metric with fewer series than columns")
}

func TestSummaryColumns(t *testing.T) {
	t.Run("app scope uses the percentile series names", func(t *testing.T) {
		summary := map[string]metricSummary{
			"cpu": {Series: []seriesPeak{{Name: "Max"}, {Name: "P50"}, {Name: "P90"}}},
		}
		assert.Equal(t, []string{"Max", "P50", "P90"}, summaryColumns(summary))
	})

	// Scoped to a container the API names each metric's sole series after the
	// metric, so the header must not be borrowed from whichever metric came first.
	t.Run("container scope collapses to one PEAK column", func(t *testing.T) {
		summary := map[string]metricSummary{
			"cpu":    {Series: []seriesPeak{{Name: "CPU"}}},
			"memory": {Series: []seriesPeak{{Name: "Memory"}}},
		}
		assert.Equal(t, []string{"PEAK"}, summaryColumns(summary))
	})

	t.Run("skips metrics that reported nothing", func(t *testing.T) {
		summary := map[string]metricSummary{
			"cpu":    {Series: nil},
			"memory": {Series: []seriesPeak{{Name: "Max"}, {Name: "P50"}}},
		}
		assert.Equal(t, []string{"Max", "P50"}, summaryColumns(summary))
	})

	t.Run("no data at all yields no columns", func(t *testing.T) {
		assert.Empty(t, summaryColumns(map[string]metricSummary{"cpu": {}}))
	})
}

func TestResolveWindow(t *testing.T) {
	t.Run("since is measured back from now", func(t *testing.T) {
		before := time.Now().UTC()
		start, end, err := resolveWindow(resourceFlags{since: 2 * time.Hour})
		require.NoError(t, err)

		assert.WithinDuration(t, before, end, time.Minute)
		assert.WithinDuration(t, before.Add(-2*time.Hour), start, time.Minute)
	})

	t.Run("explicit start wins over since", func(t *testing.T) {
		start, end, err := resolveWindow(resourceFlags{
			since: time.Hour,
			start: "2026-08-01T00:00:00Z",
			end:   "2026-08-02T00:00:00Z",
		})
		require.NoError(t, err)

		assert.Equal(t, "2026-08-01T00:00:00Z", start.Format(time.RFC3339))
		assert.Equal(t, "2026-08-02T00:00:00Z", end.Format(time.RFC3339))
	})

	t.Run("rejects a backwards window", func(t *testing.T) {
		_, _, err := resolveWindow(resourceFlags{
			start: "2026-08-02T00:00:00Z",
			end:   "2026-08-01T00:00:00Z",
		})
		assert.ErrorContains(t, err, "must be after")
	})

	t.Run("rejects a non-positive since", func(t *testing.T) {
		_, _, err := resolveWindow(resourceFlags{since: 0})
		assert.ErrorContains(t, err, "must be positive")
	})

	t.Run("rejects a malformed timestamp", func(t *testing.T) {
		_, _, err := resolveWindow(resourceFlags{start: "yesterday"})
		assert.ErrorContains(t, err, "RFC3339")
	})
}
