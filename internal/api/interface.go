package api

import "context"

type Client interface {
	GetApps(ctx context.Context, projectID string) ([]App, error)
	GetApp(ctx context.Context, projectID, appID string) (*AppDetails, error)
	DeleteApp(ctx context.Context, projectID, appID string) error
	UpdateApp(ctx context.Context, projectID, appID string, updates map[string]any) error
	GetProjects(ctx context.Context) ([]Project, error)
	GetRuns(ctx context.Context, projectID, appID string, asyncOnly bool) ([]Run, error)
	FetchAppLogs(ctx context.Context, projectID, appID string, opts AppLogOptions) (*AppLogsResponse, error)

	// Deploy methods
	CreateApp(ctx context.Context, projectID string, payload map[string]any) (*CreateAppResponse, error)
	CreatePartnerApp(ctx context.Context, projectID string, payload map[string]any) (*CreateAppResponse, error)
	UploadZip(ctx context.Context, uploadURL string, zipPath string) error
	FetchBuildLogs(ctx context.Context, projectID, appName, buildID string) (*BuildLogsResponse, error)
	GetBuild(ctx context.Context, projectID, appID, buildID string) (*AppBuild, error)
	FetchNotifications(ctx context.Context) ([]Notification, error)
	CancelBuild(ctx context.Context, projectID, appName, buildID string) error

	// Run methods
	CreateRunApp(ctx context.Context, projectID, appID, region string) error
	RunApp(ctx context.Context, projectID, appID, region, filename string, functionName *string, imageDigest *string, hardwareConfig map[string]any, tarPath string, data map[string]any) (*RunResponse, error)
	GetRunStatus(ctx context.Context, projectID, appName, runID string) (*RunStatus, error)
	CreateBaseImage(ctx context.Context, projectID, appID, region string, payload BaseImagePayload) (string, error)

	// File operations (persistent storage)
	ListFiles(ctx context.Context, projectID, path, region string) ([]FileInfo, error)
	InitiateUpload(ctx context.Context, projectID, filePath, region string, partCount int) (*InitiateUploadResponse, error)
	UploadPart(ctx context.Context, url string, data []byte) (string, error) // Returns ETag
	CompleteUpload(ctx context.Context, projectID, filePath, uploadID, region string, parts []PartInfo) error
	GetDownloadURL(ctx context.Context, projectID, filePath, region string) (string, error)
	GetFileSize(ctx context.Context, url string) (int64, error)
	DeleteFile(ctx context.Context, projectID, filePath, region string) error
}
