package wsapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/cerebriumai/cerebrium/internal/auth"
	"github.com/cerebriumai/cerebrium/pkg/config"
	"github.com/gorilla/websocket"
)

const (
	// reconnectDelay is the delay between reconnection attempts
	reconnectDelay = 2 * time.Second
	// maxReconnectAttempts is the maximum number of consecutive reconnection attempts
	maxReconnectAttempts = 5
	// pingInterval is how often we send ping frames to keep the connection alive
	pingInterval = 10 * time.Second
	// pongTimeout is how long we wait for a pong response before considering the connection dead
	pongTimeout = 5 * time.Second
	// handshakeTimeout is how long we wait for the websocket handshake
	handshakeTimeout = 5 * time.Second
)

// client implements the Client interface using gorilla/websocket.
type client struct {
	cfg *config.Config
}

var _ Client = (*client)(nil)

// NewClient creates a new websocket API client.
func NewClient(cfg *config.Config) Client {
	return &client{
		cfg: cfg,
	}
}

// wsConnectConfig holds configuration for establishing a WebSocket connection.
type wsConnectConfig struct {
	path        string            // WebSocket endpoint path (e.g., "/ws-build-logs", "/ws-logs")
	projectID   string            // Project ID for authentication
	queryParams map[string]string // Additional query parameters (e.g., buildID, appID, runID)
	logContext  []any             // Key-value pairs for structured logging
}

// streamWithReconnect is a generic WebSocket streaming function with reconnection logic.
// It handles connection management, ping/pong keep-alive, and automatic reconnection.
func (c *client) streamWithReconnect(ctx context.Context, cfg wsConnectConfig, messageHandler func([]byte) error) error {
	reconnectAttempts := 0

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := c.streamOnce(ctx, cfg, messageHandler)
		if err == nil {
			// Clean exit (server closed connection normally)
			return nil
		}

		// Check if context was cancelled
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Handle reconnection
		reconnectAttempts++
		if reconnectAttempts > maxReconnectAttempts {
			return fmt.Errorf("max reconnection attempts (%d) exceeded: %w", maxReconnectAttempts, err)
		}

		slog.Warn("WebSocket connection lost, reconnecting",
			"attempt", reconnectAttempts,
			"maxAttempts", maxReconnectAttempts,
			"error", err,
		)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(reconnectDelay):
			// Continue to reconnect
		}
	}
}

// streamOnce handles a single WebSocket connection session.
func (c *client) streamOnce(ctx context.Context, cfg wsConnectConfig, messageHandler func([]byte) error) error {
	conn, err := c.connectWS(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close() //nolint:errcheck // Best effort close

	// Set up pong handler for connection keep-alive
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pingInterval + pongTimeout))
	})

	// Start ping ticker in background
	pingDone := make(chan struct{})
	go c.pingLoop(ctx, conn, pingDone)
	defer close(pingDone)

	// Read messages
	for {
		select {
		case <-ctx.Done():
			// Send close frame before exiting
			_ = conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			return ctx.Err()
		default:
		}

		// Set read deadline
		if err := conn.SetReadDeadline(time.Now().Add(pingInterval + pongTimeout)); err != nil {
			return fmt.Errorf("failed to set read deadline: %w", err)
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				slog.Debug("WebSocket closed normally")
				return nil
			}
			return fmt.Errorf("read error: %w", err)
		}

		if err := messageHandler(message); err != nil {
			return fmt.Errorf("message handler error: %w", err)
		}
	}
}

// connectWS establishes a WebSocket connection with the given configuration.
func (c *client) connectWS(ctx context.Context, cfg wsConnectConfig) (*websocket.Conn, error) {
	token, err := c.getAuthToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth token: %w", err)
	}

	baseURL := c.cfg.GetEnvConfig().LogStreamUrl
	wsURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid logstream URL: %w", err)
	}
	wsURL.Path = cfg.path

	query := wsURL.Query()
	query.Set("projectID", cfg.projectID)
	query.Set("token", token)
	for k, v := range cfg.queryParams {
		if v != "" {
			query.Set(k, v)
		}
	}
	wsURL.RawQuery = query.Encode()

	logArgs := append([]any{"url", wsURL.Host + wsURL.Path, "projectID", cfg.projectID}, cfg.logContext...)
	slog.Debug("Connecting to websocket", logArgs...)

	dialer := websocket.Dialer{HandshakeTimeout: handshakeTimeout}
	conn, resp, err := dialer.DialContext(ctx, wsURL.String(), nil)
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("websocket dial failed with status %d: %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("websocket dial failed: %w", err)
	}

	slog.Info("Connected to websocket", logArgs...)
	return conn, nil
}

// StreamBuildLogs implements Client.StreamBuildLogs.
func (c *client) StreamBuildLogs(ctx context.Context, projectID, buildID string, from time.Time, callback func(BuildLogMessage) error) error {
	cfg := wsConnectConfig{
		path:      "/ws-build-logs",
		projectID: projectID,
		queryParams: map[string]string{
			"buildID": buildID,
		},
		logContext: []any{"buildID", buildID},
	}
	if !from.IsZero() {
		cfg.queryParams["after"] = from.Format(time.RFC3339Nano)
	}

	return c.streamWithReconnect(ctx, cfg, func(message []byte) error {
		msg, err := c.parseBuildLogMessage(message)
		if err != nil {
			slog.Warn("Failed to parse websocket message", "error", err)
			return nil // Continue on parse errors
		}
		return callback(msg)
	})
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
	if token := c.cfg.GetServiceAccountToken(); token != "" {
		if err := auth.ValidateToken(token); err == nil {
			return token, nil
		}
		return "", fmt.Errorf("service account token has expired. Please generate a new one")
	}

	// 3. Try access token
	token := c.cfg.GetAccessToken()
	if token == "" {
		return "", fmt.Errorf("no access token found. Please run 'cerebrium login' or provide a service account token")
	}

	// Check if access token is still valid
	if err := auth.ValidateToken(token); err == nil {
		return token, nil
	}

	// 4. Access token expired, try to refresh
	refreshToken := c.cfg.GetRefreshToken()
	if refreshToken == "" {
		return "", fmt.Errorf("access token has expired and no refresh token available. Please run 'cerebrium login'")
	}

	envConfig := c.cfg.GetEnvConfig()
	newToken, err := auth.RefreshToken(ctx, envConfig.AuthUrl, envConfig.ClientID, refreshToken)
	if err != nil {
		return "", fmt.Errorf("failed to refresh token: %w", err)
	}

	// Save the new token
	c.cfg.SetAccessToken(newToken)
	if err := config.Save(c.cfg); err != nil {
		return "", fmt.Errorf("failed to save new token: %w", err)
	}

	return newToken, nil
}

// pingLoop sends periodic ping messages to keep the connection alive.
func (c *client) pingLoop(ctx context.Context, conn *websocket.Conn, done chan struct{}) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(pongTimeout)); err != nil {
				slog.Debug("Failed to send ping", "error", err)
				return
			}
		}
	}
}

// parseBuildLogMessage parses a raw websocket message into a BuildLogMessage.
func (c *client) parseBuildLogMessage(data []byte) (BuildLogMessage, error) {
	var raw rawBuildLogMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return BuildLogMessage{}, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return BuildLogMessage{
		BuildID:    raw.BuildID,
		AppID:      raw.AppID,
		Timestamp:  parseTimestamp(raw.Timestamp),
		Stream:     raw.Stream,
		Log:        raw.Log,
		LineNumber: raw.LineNumber,
		Stage:      raw.Stage,
	}, nil
}

// StreamAppLogs implements Client.StreamAppLogs.
func (c *client) StreamAppLogs(ctx context.Context, projectID, appID string, opts AppLogStreamOptions, callback func(AppLogMessage) error) error {
	cfg := wsConnectConfig{
		path:      "/ws-logs",
		projectID: projectID,
		queryParams: map[string]string{
			"appID":       appID,
			"containerID": opts.ContainerID,
			"runID":       opts.RunID,
		},
		logContext: []any{"appID", appID, "runID", opts.RunID},
	}
	if !opts.From.IsZero() {
		cfg.queryParams["after"] = opts.From.Format(time.RFC3339Nano)
	}

	return c.streamWithReconnect(ctx, cfg, func(message []byte) error {
		msg, err := c.parseAppLogMessage(message)
		if err != nil {
			slog.Warn("Failed to parse websocket message", "error", err)
			return nil // Continue on parse errors
		}
		return callback(msg)
	})
}

// parseAppLogMessage parses a raw websocket message into an AppLogMessage.
func (c *client) parseAppLogMessage(data []byte) (AppLogMessage, error) {
	var raw rawAppLogMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return AppLogMessage{}, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return AppLogMessage{
		AppID:         raw.AppID,
		RunID:         raw.RunID,
		Timestamp:     parseTimestamp(raw.Timestamp),
		ContainerName: raw.ContainerName,
		Log:           raw.Log,
	}, nil
}
