package api

import (
	"fmt"
	"strconv"
	"time"
)

// App represents a Cerebrium application (used in list)
type App struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updatedAt"`
	CreatedAt time.Time `json:"createdAt"`
}

// AppDetails represents detailed information about a Cerebrium application (used in get)
// All numeric fields are returned as strings from the API
type AppDetails struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	// Hardware configuration - API returns these as strings
	Hardware string `json:"hardware"` // e.g. "CPU", "GPU"
	CPU      string `json:"cpu"`      // e.g. "2"
	Memory   string `json:"memory"`   // e.g. "8"
	GPUCount string `json:"gpuCount"` // e.g. "0"

	// Scaling parameters - API returns these as strings
	CooldownPeriodSeconds      string `json:"cooldownPeriodSeconds"`      // e.g. "1"
	MinReplicaCount            string `json:"minReplicaCount"`            // e.g. "0"
	MaxReplicaCount            string `json:"maxReplicaCount"`            // e.g. "2"
	ResponseGracePeriodSeconds string `json:"responseGracePeriodSeconds"` // e.g. "900"

	// Status
	Status          string   `json:"status"`
	LastBuildStatus string   `json:"lastBuildStatus"`
	LatestBuildID   string   `json:"latestBuildId,omitempty"`
	Pods            []string `json:"pods,omitempty"`
}

// GetCPU returns the CPU value as an integer
func (a *AppDetails) GetCPU() (int, error) {
	if a.CPU == "" {
		return 0, nil
	}
	val, err := strconv.Atoi(a.CPU)
	if err != nil {
		return 0, fmt.Errorf("failed to parse CPU value '%s': %w", a.CPU, err)
	}
	return val, nil
}

// GetMemory returns the Memory value as an integer
func (a *AppDetails) GetMemory() (int, error) {
	if a.Memory == "" {
		return 0, nil
	}
	val, err := strconv.Atoi(a.Memory)
	if err != nil {
		return 0, fmt.Errorf("failed to parse memory value '%s': %w", a.Memory, err)
	}
	return val, nil
}

// GetGPUCount returns the GPU count as an integer
func (a *AppDetails) GetGPUCount() (int, error) {
	if a.GPUCount == "" {
		return 0, nil
	}
	val, err := strconv.Atoi(a.GPUCount)
	if err != nil {
		return 0, fmt.Errorf("failed to parse GPU count value '%s': %w", a.GPUCount, err)
	}
	return val, nil
}

// GetCooldownPeriodSeconds returns the cooldown period as an integer
func (a *AppDetails) GetCooldownPeriodSeconds() (int, error) {
	if a.CooldownPeriodSeconds == "" {
		return 0, nil
	}
	val, err := strconv.Atoi(a.CooldownPeriodSeconds)
	if err != nil {
		return 0, fmt.Errorf("failed to parse cooldown period value '%s': %w", a.CooldownPeriodSeconds, err)
	}
	return val, nil
}

// GetMinReplicaCount returns the minimum replica count as an integer
func (a *AppDetails) GetMinReplicaCount() (int, error) {
	if a.MinReplicaCount == "" {
		return 0, nil
	}
	val, err := strconv.Atoi(a.MinReplicaCount)
	if err != nil {
		return 0, fmt.Errorf("failed to parse min replica count value '%s': %w", a.MinReplicaCount, err)
	}
	return val, nil
}

// GetMaxReplicaCount returns the maximum replica count as an integer
func (a *AppDetails) GetMaxReplicaCount() (int, error) {
	if a.MaxReplicaCount == "" {
		return 0, nil
	}
	val, err := strconv.Atoi(a.MaxReplicaCount)
	if err != nil {
		return 0, fmt.Errorf("failed to parse max replica count value '%s': %w", a.MaxReplicaCount, err)
	}
	return val, nil
}

// GetResponseGracePeriodSeconds returns the response grace period as an integer
func (a *AppDetails) GetResponseGracePeriodSeconds() (int, error) {
	if a.ResponseGracePeriodSeconds == "" {
		return 0, nil
	}
	val, err := strconv.Atoi(a.ResponseGracePeriodSeconds)
	if err != nil {
		return 0, fmt.Errorf("failed to parse response grace period value '%s': %w", a.ResponseGracePeriodSeconds, err)
	}
	return val, nil
}

// Project represents a Cerebrium project
type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Message string `json:"message"`
}

// CreateAppResponse represents the response from creating a new app/build
type CreateAppResponse struct {
	BuildID          string `json:"buildId"`
	Status           string `json:"status"`
	UploadURL        string `json:"uploadUrl"`
	KeyName          string `json:"keyName"`
	InternalEndpoint string `json:"internalEndpoint"`
	DashboardURL     string `json:"dashboardUrl"`
}

// BuildLog represents a single log entry
type BuildLog struct {
	CreatedAt string `json:"createdAt"`
	Log       string `json:"log"`
}

// BuildLogsResponse represents the response from fetching build logs
type BuildLogsResponse struct {
	Logs   []BuildLog `json:"logs"`
	Status string     `json:"status"`
}

// AppLogEntry represents a single app runtime log entry
type AppLogEntry struct {
	AppID         string `json:"appId"`
	ProjectID     string `json:"projectId"`
	RunID         string `json:"runId"`
	ContainerID   string `json:"containerId"`
	ContainerName string `json:"containerName"`
	LogID         string `json:"logId"`
	LineNumber    int    `json:"lineNumber"`
	LogLine       string `json:"logLine"`
	Stream        string `json:"stream"` // "stdout" or "stderr"
	Timestamp     string `json:"timestamp"`
}

// AppLogsResponse represents the response from fetching app logs
type AppLogsResponse struct {
	Logs          []AppLogEntry `json:"logs"`
	NextPageToken *string       `json:"nextPageToken"`
	HasMore       bool          `json:"hasMore"`
}

// AppLogOptions contains optional parameters for fetching app logs
type AppLogOptions struct {
	AfterDate string // ISO timestamp (optional)
	RunID     string // Filter by run (optional)
}

// Notification represents a user notification
type Notification struct {
	Message  string `json:"message"`
	Link     string `json:"link,omitempty"`
	LinkText string `json:"linkText,omitempty"`
}

// FileInfo represents a file or folder in persistent storage
type FileInfo struct {
	Name         string `json:"name"`
	IsFolder     bool   `json:"is_folder"`
	SizeBytes    int64  `json:"size_bytes"`
	LastModified string `json:"last_modified"` // ISO timestamp string
}

// InitiateUploadResponse represents the response from initiating a multipart upload
type InitiateUploadResponse struct {
	UploadID string    `json:"upload_id"`
	Parts    []PartURL `json:"parts"`
}

// PartURL represents a presigned URL for uploading a part
type PartURL struct {
	PartNumber int    `json:"part_number"`
	URL        string `json:"url"`
}

// PartInfo represents information about an uploaded part
type PartInfo struct {
	PartNumber int    `json:"part_number"`
	ETag       string `json:"etag"`
}

// DownloadURLResponse represents the response from getting a download URL
type DownloadURLResponse struct {
	URL string `json:"url"`
}

// Run represents a single run for an app
type Run struct {
	ID           string    `json:"id"`
	FunctionName string    `json:"functionName"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"createdAt"`
	IsAsync      bool      `json:"isAsync"`
}

// RunsListResponse represents the response from listing runs
type RunsListResponse struct {
	Items []Run `json:"items"`
}

// RunResponse represents the response from running an app
type RunResponse struct {
	RunID string `json:"runId"`
}

// RunStatus represents the status of a run
type RunStatus struct {
	Item struct {
		Status string `json:"status"`
	} `json:"item"`
}

// RunLog represents a single run log entry
type RunLog struct {
	LogID     string `json:"logId"`
	Timestamp string `json:"timestamp"`
	LogLine   string `json:"logLine"`
}

// RunLogsResponse represents the response from fetching run logs
type RunLogsResponse struct {
	Logs          []RunLog `json:"logs"`
	NextPageToken string   `json:"nextPageToken"`
}
