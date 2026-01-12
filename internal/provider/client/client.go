package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	BaseURL        = "https://developers.hostinger.com/api/hosting/v1"
	DefaultTimeout = 30 * time.Second
)

// Client represents the Hostinger API client
type Client struct {
	APIToken   string
	HTTPClient *http.Client
	BaseURL    string
}

// Website represents a Hostinger website
type Website struct {
	Domain         string `json:"domain"`
	OrderID        int    `json:"order_id"`
	DatacenterCode string `json:"datacenter_code,omitempty"`
	VhostType      string `json:"vhost_type,omitempty"`
	IsEnabled      bool   `json:"is_enabled,omitempty"`
	Username       string `json:"username,omitempty"`
	ClientID       int    `json:"client_id,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
	RootDirectory  string `json:"root_directory,omitempty"`
	ParentDomain   string `json:"parent_domain,omitempty"`
}

// Order represents a Hostinger hosting order
type Order struct {
	ID             int    `json:"id"`
	ClientID       int    `json:"client_id"`
	SubscriptionID string `json:"subscription_id"`
	CreatedAt      string `json:"created_at"`
	Plan           struct {
		Name string `json:"name"`
	} `json:"plan"`
	Status string `json:"status"`
}

// OrdersResponse represents the response from the orders API
type OrdersResponse struct {
	Data []Order `json:"data"`
	Meta struct {
		CurrentPage int `json:"current_page"`
		PerPage     int `json:"per_page"`
		Total       int `json:"total"`
	} `json:"meta"`
}

// WebsitesResponse represents the response from the websites API
type WebsitesResponse struct {
	Data []Website `json:"data"`
	Meta struct {
		CurrentPage int `json:"current_page"`
		PerPage     int `json:"per_page"`
		Total       int `json:"total"`
	} `json:"meta"`
}

// CreateWebsiteResponse represents the response from creating a website
type CreateWebsiteResponse struct {
	Message string `json:"message"`
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	CorrelationID string                 `json:"correlation_id"`
	Message       string                 `json:"message"`
	Errors        map[string]interface{} `json:"errors,omitempty"`
}

// NewClient creates a new Hostinger API client
func NewClient(apiToken string) *Client {
	return &Client{
		APIToken: apiToken,
		HTTPClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		BaseURL: BaseURL,
	}
}

// makeRequest performs an HTTP request to the Hostinger API
func (c *Client) makeRequest(method, endpoint string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, c.BaseURL+endpoint, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.APIToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	return resp, nil
}

// handleErrorResponse processes error responses from the API
func handleErrorResponse(resp *http.Response) error {
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("HTTP %d: failed to read error response", resp.StatusCode)
	}

	var errResp ErrorResponse
	if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return fmt.Errorf("API error (correlation_id: %s): %s", errResp.CorrelationID, errResp.Message)
}

// CreateWebsite creates a new website on Hostinger
func (c *Client) CreateWebsite(domain string, orderID int, datacenterCode string) error {
	website := Website{
		Domain:         domain,
		OrderID:        orderID,
		DatacenterCode: datacenterCode,
	}

	resp, err := c.makeRequest("POST", "/websites", website)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return handleErrorResponse(resp)
	}

	return nil
}

// GetWebsite retrieves information about a specific website
func (c *Client) GetWebsite(domain string) (*Website, error) {
	// Hostinger API doesn't have a direct GET /websites/{domain} endpoint
	// We need to list all websites and find the one we're looking for
	websites, err := c.ListWebsites()
	if err != nil {
		return nil, err
	}

	for _, website := range websites {
		if website.Domain == domain {
			return &website, nil
		}
	}

	return nil, fmt.Errorf("website %s not found", domain)
}

// DeleteWebsite deletes a website from Hostinger
func (c *Client) DeleteWebsite(domain string) error {
	resp, err := c.makeRequest("DELETE", "/websites/"+domain, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return handleErrorResponse(resp)
	}

	return nil
}

// ListWebsites retrieves all websites
func (c *Client) ListWebsites() ([]Website, error) {
	var allWebsites []Website
	page := 1
	perPage := 100

	for {
		url := fmt.Sprintf("/websites?page=%d&per_page=%d", page, perPage)
		resp, err := c.makeRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, handleErrorResponse(resp)
		}

		var websitesResp WebsitesResponse
		if err := json.NewDecoder(resp.Body).Decode(&websitesResp); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		allWebsites = append(allWebsites, websitesResp.Data...)

		// Check if we've retrieved all pages
		totalPages := (websitesResp.Meta.Total + websitesResp.Meta.PerPage - 1) / websitesResp.Meta.PerPage
		if websitesResp.Meta.CurrentPage >= totalPages || len(websitesResp.Data) == 0 {
			break
		}

		page++
	}

	return allWebsites, nil
}

// ListOrders retrieves all hosting orders
func (c *Client) ListOrders() ([]Order, error) {
	resp, err := c.makeRequest("GET", "/orders", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, handleErrorResponse(resp)
	}

	var ordersResp OrdersResponse
	if err := json.NewDecoder(resp.Body).Decode(&ordersResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return ordersResp.Data, nil
}
