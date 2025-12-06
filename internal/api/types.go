package api

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type NullableTime struct {
	time.Time
}

func (nt *NullableTime) UnmarshalJSON(b []byte) error {
	s := string(b)
	// Handle null, empty string, or missing
	if s == "null" || s == `""` || s == "" {
		return nil
	}

	// Try parsing with different formats as needed
	t, err := time.Parse(`"`+time.RFC3339+`"`, s)
	if err != nil {
		t, err = time.Parse(`"`+time.RFC3339Nano+`"`, s)
		if err != nil {
			return err
		}
	}
	nt.Time = t
	return nil
}

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
	ID        string `json:"id,omitempty"`
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
	ContainerID string // Filter by container (optional)
	BeforeDate  string // ISO timestamp - get logs before this time (optional)
	AfterDate   string // ISO timestamp - get logs after this time (optional)
	PageSize    int32  // Number of logs per page (optional, server default applies)
	NextToken   string // Pagination token for next page (optional)
	Direction   string // "forward" or "backward" (optional, affects log ordering)
	SearchTerm  string // Filter logs by search term (optional)
	Stream      string // "stdout" or "stderr" (optional)
	RunID       string // Filter by run (optional)
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

type ListRunsResponse struct {
	Items     []Run  `json:"items"`
	NextToken string `json:"nextToken"`
}

type Run struct {
	Async                   bool         `json:"async"`
	CompletedAt             NullableTime `json:"completedAt,omitempty"`
	ContainerId             string       `json:"containerId"`
	ContainerStartedAt      string       `json:"containerStartedAt"`
	CreatedAt               time.Time    `json:"createdAt"`
	FunctionName            string       `json:"functionName"`
	ID                      string       `json:"id"`
	IP                      string       `json:"ip"`
	Method                  string       `json:"method"`
	ModelId                 string       `json:"modelId"`
	ProjectId               string       `json:"projectId"`
	QueueProxyReceivedAt    NullableTime `json:"queueProxyReceivedAt,omitempty"`
	QueueProxyWaitEndedAt   NullableTime `json:"queueProxyWaitEndedAt,omitempty"`
	QueueProxyWaitStartedAt NullableTime `json:"queueProxyWaitStartedAt,omitempty"`
	QueueTimeMs             float64      `json:"queueTimeMs"`
	Region                  string       `json:"region"`
	ResponseTimeMs          float64      `json:"responseTimeMs"`
	RuntimeMs               float64      `json:"runtimeMs"`
	Status                  string       `json:"status"`
	StatusCode              *int         `json:"statusCode,omitempty"` // Use pointer to allow nil values
	UpdatedAt               NullableTime `json:"updatedAt,omitempty"`
	WebhookEndpoint         string       `json:"webhookEndpoint"`
	Websocket               bool         `json:"websocket"`

	// Calculated timings
	ActivatorQueueTimeMs int `json:"activatorQueueTimeMs"`
	ContainerQueueTimeMs int `json:"containerQueueTimeMs"`
	TotalQueueTimeMs     int `json:"totalQueueTimeMs"`
	ExecutionTimeMs      int `json:"executionTimeMs"`
	TotalResponseTimeMs  int `json:"totalResponseTimeMs"`
}

// GetDisplayStatus returns the formatted status string for display.
// Follows the logic from the TypeScript implementation with human-readable status:
// - Always prefer statusCode when available
// - Handle special status codes: -1 = closed, 0 = cancelled
// - Map 2xx codes to 'success', 5xx codes to 'failure'
// - Apply status text transformations for queued/pending states
// - Always return lowercase status
func (r *Run) GetDisplayStatus() string {
	// Always prefer status code when available
	if r.StatusCode != nil {
		code := *r.StatusCode
		if code == -1 {
			return "closed"
		} else if code == 0 {
			return "cancelled"
		} else if code >= 200 && code < 300 {
			return "success"
		} else if code >= 500 && code < 600 {
			return "failure"
		}
		// For other codes (1xx, 3xx, 4xx), show the code number
		return strconv.Itoa(code)
	}

	// Only use status text when no status code is available
	status := r.Status
	if status == "" {
		return "unknown"
	}

	// Handle special status text cases
	switch status {
	case "containerQueued", "proxyQueued":
		return "queued"
	case "pending", "processing":
		return strings.ToLower(status)
	default:
		return strings.ToLower(status)
	}
}

// AppBuild represents a build for a Cerebrium application
type AppBuild struct {
	Id        string `json:"id"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// BaseImagePayload represents the payload for creating a base image
type BaseImagePayload struct {
	Dependencies     map[string]any `json:"dependencies"`
	PreBuildCommands []string       `json:"preBuildCommands"`
	ShellCommands    []string       `json:"shellCommands"`
	BaseImageURI     string         `json:"baseImageURI,omitempty"`
}

// BaseImageResponse represents the response from creating a base image
type BaseImageResponse struct {
	Status string `json:"status"`
	Digest string `json:"digest"`
}
