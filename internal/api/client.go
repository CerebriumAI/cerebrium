package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/bugsnag/bugsnag-go/v2"
	"github.com/cerebriumai/cerebrium/internal/auth"
	"github.com/cerebriumai/cerebrium/internal/version"
	cerebrium_bugsnag "github.com/cerebriumai/cerebrium/pkg/bugsnag"
	"github.com/cerebriumai/cerebrium/pkg/config"
)

// Client is the Cerebrium API client
type client struct {
	config     *config.Config
	httpClient *http.Client
}

var _ Client = (*client)(nil)

// NewClient creates a new API client
func NewClient(cfg *config.Config) (Client, error) {
	return &client{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// getAuthToken retrieves the authentication token (service account or user token)
func (c *client) getAuthToken(ctx context.Context) (string, error) {
	// 1. Try service account token from environment variable first
	serviceToken, err := config.GetServiceAccountTokenFromEnv()
	if err != nil {
		return "", fmt.Errorf("service account token error: %w", err)
	}
	if serviceToken != "" {
		return serviceToken, nil
	}

	// 2. Try stored service account token
	if token := c.config.GetServiceAccountToken(); token != "" {
		if err := auth.ValidateToken(token); err == nil {
			return token, nil
		}
		return "", fmt.Errorf("service account token has expired. Please generate a new one")
	}

	// 3. Try access token
	token := c.config.GetAccessToken()
	if token == "" {
		return "", fmt.Errorf("no access token found. Please run 'cerebrium login' or provide a service account token")
	}

	// Check if access token is still valid
	if err := auth.ValidateToken(token); err == nil {
		return token, nil
	}

	// 4. Access token expired, try to refresh
	refreshToken := c.config.GetRefreshToken()
	if refreshToken == "" {
		return "", fmt.Errorf("access token has expired and no refresh token available. Please run 'cerebrium login'")
	}

	envConfig := c.config.GetEnvConfig()
	newToken, err := auth.RefreshToken(ctx, envConfig.AuthUrl, envConfig.ClientID, refreshToken)
	if err != nil {
		return "", fmt.Errorf("failed to refresh token: %w", err)
	}

	// Save the new token
	c.config.SetAccessToken(newToken)
	if err := config.Save(c.config); err != nil {
		return "", fmt.Errorf("failed to save new token: %w", err)
	}

	return newToken, nil
}

// request makes an HTTP request to the Cerebrium API with retry logic
func (c *client) request(ctx context.Context, method, path string, body any, requiresAuth bool) ([]byte, error) {
	var respBody []byte
	attempt := 0

	err := retry.Do(
		func() error {
			attempt++

			// Construct full URL
			reqURL := c.config.GetEnvConfig().APIV2Url + "/" + path

			// Log request details
			slog.Debug("API request",
				"method", method,
				"path", path,
				"url", reqURL,
				"requiresAuth", requiresAuth,
				"attempt", attempt,
			)

			// Marshal body if present
			var bodyReader io.Reader
			if body != nil {
				jsonBody, err := json.Marshal(body)
				if err != nil {
					slog.Error("Failed to marshal request body", "error", err, "path", path)
					return retry.Unrecoverable(fmt.Errorf("failed to marshal request body: %w", err))
				}
				bodyReader = bytes.NewBuffer(jsonBody)
				slog.Debug("Request body marshalled", "size", len(jsonBody))
			}

			// Create request with context
			req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
			if err != nil {
				slog.Error("Failed to create HTTP request", "error", err, "method", method, "url", reqURL)
				return retry.Unrecoverable(fmt.Errorf("failed to create request: %w", err))
			}

			// Set headers
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Source", "cli")
			req.Header.Set("X-CLI-Version", version.Version)

			// Add authentication if required
			if requiresAuth {
				token, err := c.getAuthToken(ctx)
				if err != nil {
					slog.Error("Failed to get auth token", "error", err)
					return retry.Unrecoverable(err)
				}
				req.Header.Set("Authorization", "Bearer "+token)
			}

			// Make request
			startTime := time.Now()
			resp, err := c.httpClient.Do(req)
			duration := time.Since(startTime)

			if err != nil {
				slog.Warn("HTTP request failed",
					"error", err,
					"method", method,
					"path", path,
					"duration", duration,
					"attempt", attempt,
				)
				return fmt.Errorf("request failed: %w", err)
			}
			defer resp.Body.Close() //nolint:errcheck // Deferred close, error not actionable

			respBody, err = io.ReadAll(resp.Body)
			if err != nil {
				slog.Error("Failed to read response body", "error", err, "statusCode", resp.StatusCode)
				return fmt.Errorf("failed to read response: %w", err)
			}

			slog.Debug("API response",
				"statusCode", resp.StatusCode,
				"responseSize", len(respBody),
				"duration", duration,
				"method", method,
				"path", path,
			)
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				slog.Info("API request successful",
					"method", method,
					"path", path,
					"statusCode", resp.StatusCode,
					"duration", duration,
				)
				return nil // Success
			}

			// Handle authentication errors specially
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				slog.Warn("Authentication failed", "statusCode", resp.StatusCode, "path", path)
				authErr := fmt.Errorf("you must log in to use this functionality. Please run 'cerebrium login'")

				// Report authentication errors to Bugsnag
				cerebrium_bugsnag.NotifyWithMetadata(
					ctx,
					authErr,
					bugsnag.SeverityWarning,
					bugsnag.MetaData{
						"api": {
							"status_code": resp.StatusCode,
							"method":      method,
							"path":        path,
							"attempt":     attempt,
						},
					},
				)

				return retry.Unrecoverable(authErr)
			}

			// Handle other errors
			var errResp ErrorResponse
			if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Message != "" {
				slog.Error("API error",
					"statusCode", resp.StatusCode,
					"message", errResp.Message,
					"path", path,
					"method", method,
				)

				apiErr := fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Message)

				// Report API errors to Bugsnag (skip 404s as they're often expected)
				if resp.StatusCode != 404 {
					severity := bugsnag.SeverityError
					if resp.StatusCode >= 500 {
						severity = bugsnag.SeverityError // Server errors
					} else if resp.StatusCode == 402 {
						severity = bugsnag.SeverityWarning // Payment required
					}

					cerebrium_bugsnag.NotifyWithMetadata(
						ctx,
						apiErr,
						severity,
						bugsnag.MetaData{
							"api": {
								"status_code": resp.StatusCode,
								"method":      method,
								"path":        path,
								"message":     errResp.Message,
								"attempt":     attempt,
							},
						},
					)
				}

				return retry.Unrecoverable(apiErr)
			}

			slog.Error("API error",
				"statusCode", resp.StatusCode,
				"response", string(respBody),
				"path", path,
				"method", method,
			)

			apiErr := fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))

			// Report unexpected API errors to Bugsnag
			if resp.StatusCode != 404 {
				cerebrium_bugsnag.NotifyWithMetadata(
					ctx,
					apiErr,
					bugsnag.SeverityError,
					bugsnag.MetaData{
						"api": {
							"status_code": resp.StatusCode,
							"method":      method,
							"path":        path,
							"response":    string(respBody),
							"attempt":     attempt,
						},
					},
				)
			}

			return retry.Unrecoverable(apiErr)
		},
		retry.Attempts(2),
		retry.LastErrorOnly(true),
	)
	if err != nil {
		return nil, err
	}

	return respBody, nil
}

// GetProjects retrieves the list of projects for the authenticated user
func (c *client) GetProjects(ctx context.Context) ([]Project, error) {
	body, err := c.request(ctx, "GET", "v2/projects", nil, true)
	if err != nil {
		return nil, err
	}

	var projects []Project
	if err := json.Unmarshal(body, &projects); err != nil {
		return nil, fmt.Errorf("failed to parse projects response: %w", err)
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Name < projects[j].Name
	})

	return projects, nil
}

// GetApps retrieves the list of apps for a specific project
func (c *client) GetApps(ctx context.Context, projectID string) ([]App, error) {
	path := fmt.Sprintf("v2/projects/%s/apps", projectID)
	body, err := c.request(ctx, "GET", path, nil, true)
	if err != nil {
		return nil, err
	}

	var apps []App
	if err := json.Unmarshal(body, &apps); err != nil {
		return nil, fmt.Errorf("failed to parse apps response: %w", err)
	}

	// Sort by most recently updated
	sort.Slice(apps, func(i, j int) bool {
		return apps[i].UpdatedAt.After(apps[j].UpdatedAt)
	})

	return apps, nil
}

// GetApp retrieves detailed information about a specific app
func (c *client) GetApp(ctx context.Context, projectID, appID string) (*AppDetails, error) {
	path := fmt.Sprintf("v2/projects/%s/apps/%s", projectID, appID)
	body, err := c.request(ctx, "GET", path, nil, true)
	if err != nil {
		return nil, err
	}

	var appDetails AppDetails
	if err := json.Unmarshal(body, &appDetails); err != nil {
		return nil, fmt.Errorf("failed to parse app details response: %w", err)
	}

	return &appDetails, nil
}

// DeleteApp deletes a specific app
func (c *client) DeleteApp(ctx context.Context, projectID, appID string) error {
	path := fmt.Sprintf("v2/projects/%s/apps/%s", projectID, appID)
	_, err := c.request(ctx, "DELETE", path, nil, true)
	return err
}

// UpdateApp updates app configuration (scaling parameters)
func (c *client) UpdateApp(ctx context.Context, projectID, appID string, updates map[string]any) error {
	path := fmt.Sprintf("v2/projects/%s/apps/%s", projectID, appID)
	_, err := c.request(ctx, "PATCH", path, updates, true)
	return err
}

// CreateApp creates a new app/build
func (c *client) CreateApp(ctx context.Context, projectID string, payload map[string]any) (*CreateAppResponse, error) {
	path := fmt.Sprintf("v2/projects/%s/apps", projectID)
	body, err := c.request(ctx, "POST", path, payload, true)
	if err != nil {
		return nil, err
	}

	var response CreateAppResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse create app response: %w", err)
	}

	return &response, nil
}

// UploadZip uploads a zip file to the given URL
func (c *client) UploadZip(ctx context.Context, uploadURL string, zipPath string) error {
	slog.Debug("Starting zip upload", "zipPath", zipPath, "uploadURL", uploadURL)

	// Open the zip file
	file, err := os.Open(zipPath) //nolint:gosec // File path from user input (deployment artifact)
	if err != nil {
		slog.Error("Failed to open zip file", "error", err, "path", zipPath)
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer file.Close() //nolint:errcheck // Deferred close, error not actionable

	// Get file info for size
	fileInfo, err := file.Stat()
	if err != nil {
		slog.Error("Failed to stat zip file", "error", err, "path", zipPath)
		return fmt.Errorf("failed to stat zip file: %w", err)
	}

	slog.Info("Uploading zip file", "size", fileInfo.Size(), "path", zipPath)

	// Create PUT request with context
	req, err := http.NewRequestWithContext(ctx, "PUT", uploadURL, file)
	if err != nil {
		slog.Error("Failed to create upload request", "error", err)
		return fmt.Errorf("failed to create upload request: %w", err)
	}

	req.Header.Set("Content-Type", "application/zip")
	req.ContentLength = fileInfo.Size()

	// Execute request
	startTime := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		slog.Error("Zip upload request failed", "error", err, "duration", duration)
		return fmt.Errorf("upload failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // Deferred close, error not actionable

	if resp.StatusCode != 200 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("Failed to read error response", "error", err, "statusCode", resp.StatusCode)
			return fmt.Errorf("failed to read error response body: %w", err)
		}
		slog.Error("Zip upload failed",
			"statusCode", resp.StatusCode,
			"response", string(body),
			"duration", duration,
		)
		return fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	slog.Info("Zip upload successful", "size", fileInfo.Size(), "duration", duration)
	return nil
}

// FetchBuildLogs retrieves build logs for a specific build
func (c *client) FetchBuildLogs(ctx context.Context, projectID, appName, buildID string) (*BuildLogsResponse, error) {
	path := fmt.Sprintf("v2/projects/%s/apps/%s-%s/builds/%s/logs", projectID, projectID, appName, buildID)
	body, err := c.request(ctx, "GET", path, nil, true)
	if err != nil {
		return nil, err
	}

	var response BuildLogsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse build logs response: %w", err)
	}

	return &response, nil
}

// GetBuild retrieves the status and details of a specific build
func (c *client) GetBuild(ctx context.Context, projectID, appID, buildID string) (*AppBuild, error) {
	path := fmt.Sprintf("v2/projects/%s/apps/%s/builds/%s", projectID, appID, buildID)
	body, err := c.request(ctx, "GET", path, nil, true)
	if err != nil {
		return nil, err
	}

	var response AppBuild
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse build response: %w", err)
	}

	return &response, nil
}

// FetchAppLogs retrieves runtime logs for a specific app
func (c *client) FetchAppLogs(ctx context.Context, projectID, appID string, opts AppLogOptions) (*AppLogsResponse, error) {
	// Build path with query parameters
	path := fmt.Sprintf("v2/projects/%s/apps/%s/logs", projectID, appID)

	// Add query parameters if provided
	params := url.Values{}
	if opts.ContainerID != "" {
		params.Set("container", opts.ContainerID)
	}
	if opts.BeforeDate != "" {
		params.Set("beforeDate", opts.BeforeDate)
	}
	if opts.AfterDate != "" {
		params.Set("afterDate", opts.AfterDate)
	}
	if opts.PageSize > 0 {
		params.Set("pageSize", fmt.Sprintf("%d", opts.PageSize))
	}
	if opts.NextToken != "" {
		params.Set("nextToken", opts.NextToken)
	}
	if opts.Direction != "" {
		params.Set("direction", opts.Direction)
	}
	if opts.SearchTerm != "" {
		params.Set("search", opts.SearchTerm)
	}
	if opts.Stream != "" {
		params.Set("stream", opts.Stream)
	}
	if opts.RunID != "" {
		params.Set("runId", opts.RunID)
	}
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	body, err := c.request(ctx, "GET", path, nil, true)
	if err != nil {
		return nil, err
	}

	// Handle 204 No Content - return empty logs response
	if len(body) == 0 {
		return &AppLogsResponse{
			Logs:          nil,
			NextPageToken: nil,
			HasMore:       false,
		}, nil
	}

	var response AppLogsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse app logs response: %w", err)
	}

	return &response, nil
}

// FetchNotifications retrieves user notifications
func (c *client) FetchNotifications(ctx context.Context) ([]Notification, error) {
	body, err := c.request(ctx, "GET", "v2/notifications", nil, false)
	if err != nil {
		return nil, err
	}

	var notifications []Notification
	if err := json.Unmarshal(body, &notifications); err != nil {
		return nil, fmt.Errorf("failed to parse notifications response: %w", err)
	}

	return notifications, nil
}

// CancelBuild cancels an ongoing build
func (c *client) CancelBuild(ctx context.Context, projectID, appName, buildID string) error {
	appID := fmt.Sprintf("%s-%s", projectID, appName)
	path := fmt.Sprintf("v2/projects/%s/apps/%s/builds/%s", projectID, appID, buildID)
	_, err := c.request(ctx, "DELETE", path, nil, true)
	return err
}

// File operation methods

// ListFiles retrieves the list of files in persistent storage
func (c *client) ListFiles(ctx context.Context, projectID, path, region string) ([]FileInfo, error) {
	queryParams := url.Values{}
	queryParams.Add("region", region)
	queryParams.Add("dir", path)
	apiPath := fmt.Sprintf("v2/projects/%s/volumes/default/ls?%s", projectID, queryParams.Encode())
	body, err := c.request(ctx, "GET", apiPath, nil, true)
	if err != nil {
		return nil, err
	}

	var files []FileInfo
	if err := json.Unmarshal(body, &files); err != nil {
		return nil, fmt.Errorf("failed to parse files response: %w", err)
	}

	return files, nil
}

// InitiateUpload initiates a multipart upload
func (c *client) InitiateUpload(ctx context.Context, projectID, filePath, region string, partCount int) (*InitiateUploadResponse, error) {
	queryParams := url.Values{}
	queryParams.Add("region", region)
	apiPath := fmt.Sprintf("v2/projects/%s/volumes/default/cp/initialize?%s", projectID, queryParams.Encode())
	payload := map[string]any{
		"file_path":  filePath,
		"part_count": partCount,
		"region":     region,
	}

	body, err := c.request(ctx, "POST", apiPath, payload, true)
	if err != nil {
		return nil, err
	}

	var response InitiateUploadResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse initiate upload response: %w", err)
	}

	return &response, nil
}

// UploadPart uploads a single part and returns the ETag
func (c *client) UploadPart(ctx context.Context, url string, data []byte) (string, error) {
	slog.Debug("Uploading part", "size", len(data), "url", url)

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(data))
	if err != nil {
		slog.Error("Failed to create part upload request", "error", err)
		return "", fmt.Errorf("failed to create upload request: %w", err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = int64(len(data))

	// Use a longer timeout for uploads (5 minutes for 50MB parts)
	uploadClient := &http.Client{
		Timeout: 5 * time.Minute,
	}

	startTime := time.Now()
	resp, err := uploadClient.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		slog.Error("Part upload failed", "error", err, "size", len(data), "duration", duration)
		return "", fmt.Errorf("upload failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // Deferred close, error not actionable

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		slog.Error("Part upload failed",
			"statusCode", resp.StatusCode,
			"response", string(body),
			"size", len(data),
			"duration", duration,
		)
		return "", fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	etag := resp.Header.Get("ETag")
	slog.Debug("Part upload successful", "size", len(data), "duration", duration, "etag", etag)
	return etag, nil
}

// CompleteUpload completes a multipart upload
func (c *client) CompleteUpload(ctx context.Context, projectID, filePath, uploadID, region string, parts []PartInfo) error {
	queryParams := url.Values{}
	queryParams.Add("region", region)
	apiPath := fmt.Sprintf("v2/projects/%s/volumes/default/cp/complete?%s", projectID, queryParams.Encode())
	payload := map[string]any{
		"upload_id": uploadID,
		"file_path": filePath,
		"parts":     parts,
		"region":    region,
	}

	_, err := c.request(ctx, "POST", apiPath, payload, true)
	return err
}

// GetDownloadURL retrieves a presigned URL for downloading a file
func (c *client) GetDownloadURL(ctx context.Context, projectID, filePath, region string) (string, error) {
	queryParams := url.Values{}
	queryParams.Add("region", region)
	queryParams.Add("file_path", filePath)
	apiPath := fmt.Sprintf("v2/projects/%s/volumes/default/download?%s", projectID, queryParams.Encode())
	body, err := c.request(ctx, "GET", apiPath, nil, true)
	if err != nil {
		return "", err
	}

	var response DownloadURLResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to parse download URL response: %w", err)
	}

	if response.URL == "" {
		return "", fmt.Errorf("no download URL in response")
	}

	return response.URL, nil
}

// DeleteFile removes a file or directory from persistent storage
func (c *client) DeleteFile(ctx context.Context, projectID, filePath, region string) error {
	queryParams := url.Values{}
	queryParams.Add("region", region)
	queryParams.Add("file_path", filePath)
	apiPath := fmt.Sprintf("v2/projects/%s/volumes/default/rm?%s", projectID, queryParams.Encode())
	_, err := c.request(ctx, "DELETE", apiPath, nil, true)
	return err
}

// CreateRunApp creates a temporary app for running
func (c *client) CreateRunApp(ctx context.Context, projectID, appID, region string) error {
	queryParams := url.Values{}
	queryParams.Add("region", region)
	path := fmt.Sprintf("v3/projects/%s/apps/%s/create-run-app?%s", projectID, appID, queryParams.Encode())
	_, err := c.request(ctx, "POST", path, nil, true)
	return err
}

// RunApp uploads and executes code
func (c *client) RunApp(ctx context.Context, projectID, appID, region, filename string, functionName *string, imageDigest *string, hardwareConfig map[string]any, tarPath string, data map[string]any) (*RunResponse, error) {
	// Build query parameters
	queryParams := url.Values{}
	queryParams.Add("filename", filename)
	queryParams.Add("appName", strings.TrimPrefix(appID, projectID+"-"))
	queryParams.Add("region", region)

	if functionName != nil {
		queryParams.Add("functionName", *functionName)
	}

	if imageDigest != nil {
		queryParams.Add("imageDigest", *imageDigest)
	}

	// Add hardware parameters
	for key, value := range hardwareConfig {
		// Format float64 values to match Python CLI behavior (e.g., "2.0" not "2")
		if f, ok := value.(float64); ok {
			s := fmt.Sprintf("%g", f)
			// Ensure at least one decimal place for whole numbers
			if !strings.Contains(s, ".") {
				s += ".0"
			}
			queryParams.Add(key, s)
		} else {
			queryParams.Add(key, fmt.Sprintf("%v", value))
		}
	}

	// Create multipart form
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	// Add projectId form field
	if err := w.WriteField("projectId", projectID); err != nil {
		return nil, fmt.Errorf("failed to write projectId field: %w", err)
	}

	// Add data JSON with Content-Type: application/json
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	dataHeader := make(textproto.MIMEHeader)
	dataHeader.Set("Content-Disposition", `form-data; name="data"; filename="data.json"`)
	dataHeader.Set("Content-Type", "application/json")
	dataWriter, err := w.CreatePart(dataHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to create data form field: %w", err)
	}
	if _, err := dataWriter.Write(dataJSON); err != nil {
		return nil, fmt.Errorf("failed to write data: %w", err)
	}

	// Add tar file with Content-Type: application/x-tar
	tarFile, err := os.Open(tarPath) //nolint:gosec // File path from user input (deployment artifact)
	if err != nil {
		return nil, fmt.Errorf("failed to open tar file: %w", err)
	}
	defer tarFile.Close() //nolint:errcheck // Deferred close, error not actionable

	tarHeader := make(textproto.MIMEHeader)
	tarHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filepath.Base(tarPath)))
	tarHeader.Set("Content-Type", "application/x-tar")
	tarWriter, err := w.CreatePart(tarHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to create tar form field: %w", err)
	}

	if _, err := io.Copy(tarWriter, tarFile); err != nil {
		return nil, fmt.Errorf("failed to copy tar file: %w", err)
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Make request
	urlString := c.config.GetEnvConfig().APIV2Url + "/v3/projects/" + projectID + "/apps/" + appID + "/run?" + queryParams.Encode()
	path := "v3/projects/" + projectID + "/apps/" + appID + "/run?" + queryParams.Encode()

	req, err := http.NewRequestWithContext(ctx, "POST", urlString, &b)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("X-Source", "cli")
	req.Header.Set("X-CLI-Version", version.Version)

	// Add authentication
	token, err := c.getAuthToken(ctx)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	// Debug logging
	slog.Debug("API request",
		"method", "POST",
		"path", path,
		"url", urlString,
		"requiresAuth", true,
		"attempt", 1,
	)

	startTime := time.Now()

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // Deferred close, error not actionable

	duration := time.Since(startTime)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Log response
	slog.Debug("API response",
		"statusCode", resp.StatusCode,
		"responseSize", len(respBody),
		"duration", duration,
		"method", "POST",
		"path", path,
	)

	if resp.StatusCode != 200 {
		slog.Warn("API request failed",
			"method", "POST",
			"path", path,
			"statusCode", resp.StatusCode,
			"duration", duration,
		)
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Message != "" {
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Message)
		}
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	slog.Info("API request successful",
		"method", "POST",
		"path", path,
		"statusCode", resp.StatusCode,
		"duration", duration,
	)

	var runResp RunResponse
	if err := json.Unmarshal(respBody, &runResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &runResp, nil
}

// GetRunStatus retrieves the status of a run
func (c *client) GetRunStatus(ctx context.Context, projectID, appName, runID string) (*RunStatus, error) {
	appID := fmt.Sprintf("%s-%s", projectID, appName)
	path := fmt.Sprintf("v2/projects/%s/apps/%s/runs/%s", projectID, appID, runID)

	body, err := c.request(ctx, "GET", path, nil, true)
	if err != nil {
		return nil, err
	}

	var status RunStatus
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, fmt.Errorf("failed to parse run status: %w", err)
	}

	return &status, nil
}

// CreateBaseImage creates a base image with dependencies, polling until ready
func (c *client) CreateBaseImage(ctx context.Context, projectID, appID, region string, payload BaseImagePayload) (string, error) {
	queryParams := url.Values{}
	queryParams.Add("region", region)
	path := fmt.Sprintf("v3/projects/%s/apps/%s/base-image?%s", projectID, appID, queryParams.Encode())

	const maxAttempts = 15
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for attempt := 0; attempt < maxAttempts; attempt++ {
		body, err := c.request(ctx, "POST", path, payload, true)
		if err != nil {
			return "", err
		}

		var resp BaseImageResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return "", fmt.Errorf("failed to parse base image response: %w", err)
		}

		if resp.Status == "ready" && resp.Digest != "" {
			return resp.Digest, nil
		}

		// Wait for next tick (except on last attempt)
		if attempt < maxAttempts-1 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-ticker.C:
				// Continue to next attempt
			}
		}
	}

	return "", fmt.Errorf("base image not ready after %d attempts", maxAttempts)
}

// GetRuns retrieves the list of runs for a specific app
func (c *client) GetRuns(ctx context.Context, projectID, appID string, asyncOnly bool) ([]Run, error) {
	path := fmt.Sprintf("v2/projects/%s/apps/%s/runs", projectID, appID)

	// Build query parameters using url.Values
	if asyncOnly {
		params := url.Values{}
		params.Set("asyncOnly", "true")
		path += "?" + params.Encode()
	}

	body, err := c.request(ctx, "GET", path, nil, true)
	if err != nil {
		return nil, err
	}

	var response ListRunsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse runs response: %w", err)
	}

	return response.Items, nil
}
