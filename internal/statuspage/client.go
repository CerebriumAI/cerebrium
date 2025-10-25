package statuspage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Status string

const (
	StatusOperational         Status = "operational"
	StatusDegradedPerformance Status = "degraded_performance"
	StatusDegraded            Status = "degraded"
	StatusDowntime            Status = "downtime"
	StatusMaintenance         Status = "maintenance"
)

// Component represents a service component on the status page
type Component struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status Status `json:"status"` // operational, degraded_performance, downtime, maintenance
}

// Incident represents an ongoing incident
type Incident struct {
	ID                 string
	Name               string
	Status             Status
	URL                string
	LastUpdateMessage  string
	CurrentWorstImpact string
	AffectedComponents []Component
}

// StatusResponse represents the normalized status response
type StatusResponse struct {
	OngoingIncidents []Incident
	AllComponents    []Component
}

// rawStatusResponse represents the raw JSON:API response from status.cerebrium.ai
type rawStatusResponse struct {
	Data     *statusPageData `json:"data"`
	Included []includedItem  `json:"included"`
}

type statusPageData struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Attributes map[string]any `json:"attributes"`
}

type includedItem struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Attributes map[string]any `json:"attributes"`
}

// HTTPClient interface for dependency injection (allows mocking)
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client is the status page client
type Client struct {
	httpClient HTTPClient
	statusURL  string
}

// NewClient creates a new status page client
func NewClient(httpClient HTTPClient) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}

	return &Client{
		httpClient: httpClient,
		statusURL:  "https://status.cerebrium.ai/index.json",
	}
}

// GetStatus fetches the current status from the API
func (c *Client) GetStatus(ctx context.Context) (*StatusResponse, error) {
	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", c.statusURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "cerebrium-cli")

	// Make the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching status: %w", err)
	}
	//nolint:errcheck // Deferred close, error not actionable
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP status: %d", resp.StatusCode)
	}

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	// Parse JSON
	var rawResp rawStatusResponse
	if err := json.Unmarshal(body, &rawResp); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %w", err)
	}

	// Normalize and return
	return c.normalize(&rawResp), nil
}

// normalize converts the raw JSON:API response to our internal format
func (c *Client) normalize(raw *rawStatusResponse) *StatusResponse {
	resp := &StatusResponse{
		OngoingIncidents: make([]Incident, 0),
		AllComponents:    make([]Component, 0),
	}

	// Extract sections to map section names
	sectionNames := make(map[string]string)
	for _, item := range raw.Included {
		if item.Type == "status_page_section" {
			if name, ok := item.Attributes["name"].(string); ok {
				sectionNames[item.ID] = name
			}
		}
	}

	// Extract components from status_page_resource items
	componentMap := make(map[string]Component)
	for _, item := range raw.Included {
		if item.Type == "status_page_resource" {
			// Try to get the public_name first, fall back to section name if available
			var name string
			if publicName, ok := item.Attributes["public_name"].(string); ok && publicName != "" {
				name = publicName
			} else {
				// Try to get section name from section_id (could be number or string)
				var sectionID string
				if id, ok := item.Attributes["status_page_section_id"].(float64); ok {
					sectionID = fmt.Sprintf("%.0f", id)
				} else if id, ok := item.Attributes["status_page_section_id"].(string); ok {
					sectionID = id
				}
				if sectionID != "" {
					if sectionName, exists := sectionNames[sectionID]; exists {
						name = sectionName
					}
				}
			}

			if name != "" {
				status := StatusOperational // default
				if s, ok := item.Attributes["status"].(Status); ok {
					status = s
				}

				component := Component{
					ID:     item.ID,
					Name:   name,
					Status: status,
				}
				componentMap[item.ID] = component
				resp.AllComponents = append(resp.AllComponents, component)
			}
		}
	}

	// Check for unresolved status reports (incidents)
	for _, item := range raw.Included {
		if item.Type == "status_report" {
			if aggregateState, ok := item.Attributes["aggregate_state"].(Status); ok && aggregateState != "resolved" {
				// This is an active incident
				incidentURL := "https://status.cerebrium.ai"
				if raw.Data != nil && raw.Data.Attributes != nil {
					if customDomain, ok := raw.Data.Attributes["custom_domain"].(string); ok && customDomain != "" {
						incidentURL = "https://" + customDomain
					}
				}

				title := "Ongoing incident"
				if titleAttr, ok := item.Attributes["title"].(string); ok && titleAttr != "" {
					title = titleAttr
				}

				// Find affected components from this incident
				var affectedComponents []Component
				if affectedResources, ok := item.Attributes["affected_resources"].([]any); ok {
					for _, res := range affectedResources {
						if resMap, ok := res.(map[string]any); ok {
							if resourceID, ok := resMap["status_page_resource_id"].(string); ok {
								if comp, exists := componentMap[resourceID]; exists {
									affectedComponents = append(affectedComponents, comp)
								}
							}
						}
					}
				}

				incident := Incident{
					ID:                 item.ID,
					Name:               title,
					Status:             aggregateState,
					URL:                incidentURL,
					CurrentWorstImpact: mapAggregateStateToImpact(aggregateState),
					AffectedComponents: affectedComponents,
				}

				resp.OngoingIncidents = append(resp.OngoingIncidents, incident)
			}
		}
	}

	// Also check overall system state as fallback
	if len(resp.OngoingIncidents) == 0 && raw.Data != nil && raw.Data.Attributes != nil {
		if overallState, ok := raw.Data.Attributes["aggregate_state"].(Status); ok && overallState != StatusOperational {
			// Find affected components (non-operational)
			var affectedComponents []Component
			for _, comp := range resp.AllComponents {
				if comp.Status != StatusOperational {
					affectedComponents = append(affectedComponents, comp)
				}
			}

			incidentURL := "https://status.cerebrium.ai"
			if customDomain, ok := raw.Data.Attributes["custom_domain"].(string); ok && customDomain != "" {
				incidentURL = "https://" + customDomain
			}

			title := "Some Cerebrium services are experiencing issues"
			if len(affectedComponents) > 0 {
				componentNames := make([]string, 0, len(affectedComponents))
				for _, comp := range affectedComponents {
					componentNames = append(componentNames, comp.Name)
				}
				title = fmt.Sprintf("Issues affecting: %s", strings.Join(componentNames, ", "))
			}

			incident := Incident{
				ID:                 "system-status",
				Name:               title,
				Status:             overallState,
				URL:                incidentURL,
				CurrentWorstImpact: mapAggregateStateToImpact(overallState),
				AffectedComponents: affectedComponents,
			}

			resp.OngoingIncidents = append(resp.OngoingIncidents, incident)
		}
	}

	// Use sections as fallback if no components found
	if len(resp.AllComponents) == 0 {
		for id, name := range sectionNames {
			resp.AllComponents = append(resp.AllComponents, Component{
				ID:     id,
				Name:   name,
				Status: StatusOperational,
			})
		}
	}

	return resp
}

// mapAggregateStateToImpact maps aggregate_state to impact level
func mapAggregateStateToImpact(state Status) string {
	switch state {
	case StatusDowntime:
		return "critical"
	case StatusDegradedPerformance, StatusDegraded:
		return "major"
	case StatusMaintenance:
		return "maintenance"
	default:
		return "minor"
	}
}
