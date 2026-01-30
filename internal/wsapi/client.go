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

// StreamBuildLogs implements Client.StreamBuildLogs.
func (c *client) StreamBuildLogs(ctx context.Context, projectID, buildID string, from time.Time, callback func(BuildLogMessage) error) error {
	reconnectAttempts := 0

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := c.streamBuildLogsOnce(ctx, projectID, buildID, from, callback)
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

// streamBuildLogsOnce handles a single websocket connection session.
func (c *client) streamBuildLogsOnce(ctx context.Context, projectID, buildID string, from time.Time, callback func(BuildLogMessage) error) error {
	conn, err := c.connect(ctx, projectID, buildID, from)
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

		msg, err := c.parseMessage(message)
		if err != nil {
			slog.Warn("Failed to parse websocket message", "error", err)
			continue
		}

		if err := callback(msg); err != nil {
			return fmt.Errorf("callback error: %w", err)
		}
	}
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

// connect establishes a websocket connection to the build logs endpoint.
func (c *client) connect(ctx context.Context, projectID, buildID string, from time.Time) (*websocket.Conn, error) {
	// Get auth token
	token, err := c.getAuthToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth token: %w", err)
	}

	// Build websocket URL with query parameters
	baseURL := c.cfg.GetEnvConfig().LogStreamUrl
	wsURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid logstream URL: %w", err)
	}
	wsURL.Path = "/ws-build-logs"

	query := wsURL.Query()
	query.Set("projectID", projectID)
	query.Set("buildID", buildID)
	query.Set("token", token)
	if !from.IsZero() {
		query.Set("after", from.Format(time.RFC3339Nano))
	}
	wsURL.RawQuery = query.Encode()

	slog.Debug("Connecting to build logs websocket",
		"url", wsURL.Host+wsURL.Path,
		"projectID", projectID,
		"buildID", buildID,
	)

	// Connect with context
	dialer := websocket.Dialer{
		HandshakeTimeout: handshakeTimeout,
	}

	conn, resp, err := dialer.DialContext(ctx, wsURL.String(), nil)
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("websocket dial failed with status %d: %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("websocket dial failed: %w", err)
	}

	// TODO(wes): Gracefully handle auth errors, like `client` does

	slog.Info("Connected to build logs websocket",
		"projectID", projectID,
		"buildID", buildID,
	)

	return conn, nil
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

// parseMessage parses a raw websocket message into a BuildLogMessage.
func (c *client) parseMessage(data []byte) (BuildLogMessage, error) {
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
	reconnectAttempts := 0

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := c.streamAppLogsOnce(ctx, projectID, appID, opts, callback)
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

// streamAppLogsOnce handles a single websocket connection session for app logs.
func (c *client) streamAppLogsOnce(ctx context.Context, projectID, appID string, opts AppLogStreamOptions, callback func(AppLogMessage) error) error {
	conn, err := c.connectAppLogs(ctx, projectID, appID, opts)
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

		msg, err := c.parseAppLogMessage(message)
		if err != nil {
			slog.Warn("Failed to parse websocket message", "error", err)
			continue
		}

		if err := callback(msg); err != nil {
			return fmt.Errorf("callback error: %w", err)
		}
	}
}

// connectAppLogs establishes a websocket connection to the app logs endpoint.
func (c *client) connectAppLogs(ctx context.Context, projectID, appID string, opts AppLogStreamOptions) (*websocket.Conn, error) {
	// Get auth token
	token, err := c.getAuthToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth token: %w", err)
	}

	// Build websocket URL with query parameters
	baseURL := c.cfg.GetEnvConfig().LogStreamUrl
	wsURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid logstream URL: %w", err)
	}
	wsURL.Path = "/ws-logs"

	query := wsURL.Query()
	query.Set("projectID", projectID)
	query.Set("appID", appID)
	query.Set("token", token)
	if !opts.From.IsZero() {
		query.Set("after", opts.From.Format(time.RFC3339Nano))
	}
	if opts.ContainerID != "" {
		query.Set("containerID", opts.ContainerID)
	}
	if opts.RunID != "" {
		query.Set("runID", opts.RunID)
	}
	wsURL.RawQuery = query.Encode()

	slog.Debug("Connecting to app logs websocket",
		"url", wsURL.Host+wsURL.Path,
		"projectID", projectID,
		"appID", appID,
		"runID", opts.RunID,
	)

	// Connect with context
	dialer := websocket.Dialer{
		HandshakeTimeout: handshakeTimeout,
	}

	conn, resp, err := dialer.DialContext(ctx, wsURL.String(), nil)
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("websocket dial failed with status %d: %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("websocket dial failed: %w", err)
	}

	slog.Info("Connected to app logs websocket",
		"projectID", projectID,
		"appID", appID,
	)

	return conn, nil
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
