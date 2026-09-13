package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetricValueUnmarshal(t *testing.T) {
	tcs := []struct {
		name      string
		input     string
		wantValid bool
		wantValue float64
		wantErr   bool
	}{
		{name: "quoted value, as the API sends it", input: `"1.0432586960842736"`, wantValid: true, wantValue: 1.0432586960842736},
		{name: "padded gap", input: `null`, wantValid: false},
		{name: "empty string", input: `""`, wantValid: false},
		{name: "bare number", input: `2.5`, wantValid: true, wantValue: 2.5},
		{name: "quoted zero is a real measurement", input: `"0"`, wantValid: true, wantValue: 0},
		{name: "quoted NaN counts as no sample", input: `"NaN"`, wantValid: false},
		{name: "quoted infinity counts as no sample", input: `"+Inf"`, wantValid: false},
		{name: "unparseable string", input: `"not-a-number"`, wantErr: true},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			var value MetricValue
			err := json.Unmarshal([]byte(tc.input), &value)

			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantValid, value.Valid)
			if tc.wantValid {
				assert.Equal(t, tc.wantValue, value.Value)
			}
		})
	}
}

// Values go out as numbers regardless of how they came in, so consumers of our
// JSON can compare them without unquoting first.
func TestMetricValueMarshalsAsNumber(t *testing.T) {
	var series ChartSeries
	require.NoError(t, json.Unmarshal([]byte(`{"name":"Max","data":["1.5",null,"2"]}`), &series))

	out, err := json.Marshal(series)
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"Max","data":[1.5,null,2]}`, string(out))
}

func TestResourceMetricsUnmarshal(t *testing.T) {
	body := `{
		"cpu": {"timestamps": [1, 2], "series": [{"name": "Max", "data": ["0.5", "1.5"]}]},
		"memory": {"timestamps": [1, 2], "series": [{"name": "Max", "data": [null, "8"]}]},
		"gpu": {"timestamps": [1, 2], "series": [{"name": "Max", "data": [null, null]}]},
		"containers": {"timestamps": [1], "series": []},
		"requests": {"timestamps": [1], "series": []}
	}`

	var metrics ResourceMetrics
	require.NoError(t, json.Unmarshal([]byte(body), &metrics))

	assert.Equal(t, []int64{1, 2}, metrics.CPU.Timestamps)
	require.Len(t, metrics.CPU.Series, 1)
	assert.Equal(t, "Max", metrics.CPU.Series[0].Name)
	assert.True(t, metrics.CPU.Series[0].Data[1].Valid)
	assert.Equal(t, 1.5, metrics.CPU.Series[0].Data[1].Value)

	assert.False(t, metrics.Memory.Series[0].Data[0].Valid)
	assert.True(t, metrics.Memory.Series[0].Data[1].Valid)

	// A GPU series that never reported stays absent rather than reading as zero.
	assert.False(t, metrics.GPU.Series[0].Data[0].Valid)
	assert.False(t, metrics.GPU.Series[0].Data[1].Valid)
}
