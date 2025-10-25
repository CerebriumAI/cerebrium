package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"time"
)

// DeviceAuthResponse represents the response from device authorization
type DeviceAuthResponse struct {
	DeviceAuthResponsePayload struct {
		DeviceCode              string `json:"device_code"`
		UserCode                string `json:"user_code"`
		VerificationURI         string `json:"verification_uri"`
		VerificationURIComplete string `json:"verification_uri_complete"`
		ExpiresIn               int    `json:"expires_in"`
		Interval                int    `json:"interval"`
	} `json:"deviceAuthResponsePayload"`
}

// TokenResponse represents the response from token endpoint
type TokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

// ErrorResponse for API errors
type ErrorResponse struct {
	Message   string `json:"message"`
	Error     string `json:"error"`
	ErrorCode string `json:"error_code"` // Some endpoints use error_code instead of error
}

// RequestDeviceCode initiates the device authorization flow
func RequestDeviceCode(ctx context.Context, apiURL string) (*DeviceAuthResponse, error) {
	url := fmt.Sprintf("%s/device-authorization", apiURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer([]byte("{}")))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Source", "cli")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck // Deferred close, error not actionable

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if json.Unmarshal(body, &errResp) == nil {
			return nil, fmt.Errorf("API error: %s", errResp.Message)
		}
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var deviceAuth DeviceAuthResponse
	if err := json.Unmarshal(body, &deviceAuth); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &deviceAuth, nil
}

// PollForToken polls the token endpoint until authentication is complete
func PollForToken(ctx context.Context, apiURL string, deviceCode string) (*TokenResponse, error) {
	url := fmt.Sprintf("%s/token", apiURL)
	payload := map[string]string{
		"device_code": deviceCode,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	startTime := time.Now()
	maxDuration := 60 * time.Second

	var ticker *time.Ticker

	for time.Since(startTime) < maxDuration {
		if ticker == nil {
			// Only wait for ticker on the second iteration
			ticker = time.NewTicker(time.Millisecond * 500)
		} else {
			<-ticker.C
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonPayload))
		if err != nil {
			return nil, err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Source", "cli")

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		body, err := io.ReadAll(resp.Body)
		//nolint:errcheck,gosec // Closing response body, error not actionable
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == http.StatusOK {
			var tokenResp TokenResponse
			if err := json.Unmarshal(body, &tokenResp); err != nil {
				return nil, fmt.Errorf("failed to parse token response: %w", err)
			}
			return &tokenResp, nil
		}

		// Check for authorization_pending (expected while waiting)
		if resp.StatusCode == http.StatusBadRequest {
			var errResp ErrorResponse
			if json.Unmarshal(body, &errResp) == nil {
				// Check both error and error_code fields
				if errResp.Error == "authorization_pending" || errResp.ErrorCode == "authorization_pending" {
					// Expected, continue polling
					continue
				}
			}
		}

		// Any other error
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("token request failed (%d): %s", resp.StatusCode, string(body))
		}
	}

	return nil, fmt.Errorf("authentication timeout - please try again")
}

// OpenBrowser opens the default browser with the given URL
func OpenBrowser(ctx context.Context, url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	default:
		return fmt.Errorf("unsupported platform")
	}

	return exec.CommandContext(ctx, cmd, args...).Start() //nolint:gosec // Browser command from OS defaults
}
