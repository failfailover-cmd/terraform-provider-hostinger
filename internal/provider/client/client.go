package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	BaseURL                   = "https://developers.hostinger.com/api/hosting/v1"
	DefaultTimeout            = 30 * time.Second
	MaxRetries                = 7
	BaseBackoff               = 2 * time.Second
	MaxBackoff                = 60 * time.Second
	DefaultMinRequestInterval = 1200 * time.Millisecond
)

// Client represents the Hostinger API client
type Client struct {
	APIToken           string
	HTTPClient         *http.Client
	BaseURL            string
	MaxRetries         int
	BaseBackoff        time.Duration
	MaxBackoff         time.Duration
	MinRequestInterval time.Duration

	mu          sync.Mutex
	lastRequest time.Time
}

type Config struct {
	APIToken           string
	MaxRetries         int
	BaseBackoff        time.Duration
	MaxBackoff         time.Duration
	MinRequestInterval time.Duration
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

// APIError is a typed error for Hostinger API responses.
type APIError struct {
	StatusCode    int
	CorrelationID string
	Message       string
	RawBody       string
}

func (e *APIError) Error() string {
	msg := strings.TrimSpace(e.Message)
	if msg == "" {
		msg = strings.TrimSpace(e.RawBody)
	}
	if msg == "" {
		msg = "unknown API error"
	}
	return fmt.Sprintf("API error (status: %d, correlation_id: %s): %s", e.StatusCode, e.CorrelationID, msg)
}

func (e *APIError) IsRateLimited() bool {
	body := strings.ToLower(e.RawBody)
	msg := strings.ToLower(e.Message)
	return e.StatusCode == http.StatusTooManyRequests ||
		strings.Contains(body, "1015") ||
		strings.Contains(body, "too many attempts") ||
		strings.Contains(msg, "too many attempts")
}

func (e *APIError) IsNotFound() bool {
	return e.StatusCode == http.StatusNotFound
}

// NewClient creates a new Hostinger API client
func NewClient(apiToken string) *Client {
	return NewClientWithConfig(Config{APIToken: apiToken})
}

// NewClientWithConfig creates a new Hostinger API client with custom retry and pacing settings.
func NewClientWithConfig(cfg Config) *Client {
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = MaxRetries
	}
	baseBackoff := cfg.BaseBackoff
	if baseBackoff <= 0 {
		baseBackoff = BaseBackoff
	}
	maxBackoff := cfg.MaxBackoff
	if maxBackoff <= 0 {
		maxBackoff = MaxBackoff
	}
	minReqInterval := cfg.MinRequestInterval
	if minReqInterval <= 0 {
		minReqInterval = DefaultMinRequestInterval
	}

	return &Client{
		APIToken: cfg.APIToken,
		HTTPClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		BaseURL:            BaseURL,
		MaxRetries:         maxRetries,
		BaseBackoff:        baseBackoff,
		MaxBackoff:         maxBackoff,
		MinRequestInterval: minReqInterval,
	}
}

// makeRequest performs an HTTP request to the Hostinger API
func (c *Client) makeRequest(method, endpoint string, body interface{}) (*http.Response, error) {
	var bodyBytes []byte
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyBytes = jsonBody
	}

	var lastErr error
	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
		var reqBody io.Reader
		if len(bodyBytes) > 0 {
			reqBody = bytes.NewBuffer(bodyBytes)
		}

		req, err := http.NewRequest(method, c.BaseURL+endpoint, reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+c.APIToken)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		c.waitTurn()
		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to execute request: %w", err)
			if attempt == c.MaxRetries {
				return nil, lastErr
			}
			time.Sleep(c.backoffWithJitter(attempt))
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}

		apiErr := readAPIError(resp)
		lastErr = apiErr

		if !isRetryableStatus(resp.StatusCode) && !apiErr.IsRateLimited() {
			return nil, apiErr
		}
		if attempt == c.MaxRetries {
			return nil, fmt.Errorf("API retry limit exceeded after %d attempts: %w", c.MaxRetries+1, apiErr)
		}

		sleepDur := c.retryAfterOrBackoff(resp.Header.Get("Retry-After"), attempt)
		time.Sleep(sleepDur)
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("request failed with unknown error")
}

func isRetryableStatus(code int) bool {
	return code == http.StatusTooManyRequests || (code >= 500 && code <= 599)
}

func (c *Client) waitTurn() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	if !c.lastRequest.IsZero() {
		elapsed := now.Sub(c.lastRequest)
		if elapsed < c.MinRequestInterval {
			time.Sleep(c.MinRequestInterval - elapsed)
		}
	}
	c.lastRequest = time.Now()
}

func (c *Client) backoffWithJitter(attempt int) time.Duration {
	d := c.BaseBackoff * (1 << attempt)
	if d > c.MaxBackoff {
		d = c.MaxBackoff
	}
	jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
	return d + jitter
}

func (c *Client) retryAfterOrBackoff(header string, attempt int) time.Duration {
	if header != "" {
		if seconds, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && seconds >= 0 {
			return time.Duration(seconds) * time.Second
		}
		if t, err := http.ParseTime(header); err == nil {
			d := time.Until(t)
			if d > 0 {
				return d
			}
		}
	}
	return c.backoffWithJitter(attempt)
}

func readAPIError(resp *http.Response) *APIError {
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    "failed to read error response",
		}
	}

	var errResp ErrorResponse
	if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
		return &APIError{
			StatusCode: resp.StatusCode,
			RawBody:    string(bodyBytes),
		}
	}

	return &APIError{
		StatusCode:    resp.StatusCode,
		CorrelationID: errResp.CorrelationID,
		Message:       errResp.Message,
		RawBody:       string(bodyBytes),
	}
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
		// If API says resource already exists, treat create as idempotent success.
		var apiErr *APIError
		if errors.As(err, &apiErr) {
			msg := strings.ToLower(apiErr.Message + " " + apiErr.RawBody)
			if strings.Contains(msg, "already") && strings.Contains(msg, "exist") {
				return nil
			}
		}
		return err
	}
	defer resp.Body.Close()

	return nil
}

// GetWebsite retrieves information about a specific website
func (c *Client) GetWebsite(domain string) (*Website, error) {
	// Hostinger API does not expose a guaranteed direct GET /websites/{domain}
	// across all accounts, so we scan pages and stop on first match.
	page := 1
	perPage := 100

	for {
		url := fmt.Sprintf("/websites?page=%d&per_page=%d", page, perPage)
		resp, err := c.makeRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}

		var websitesResp WebsitesResponse
		if err := json.NewDecoder(resp.Body).Decode(&websitesResp); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		resp.Body.Close()

		for _, website := range websitesResp.Data {
			if website.Domain == domain {
				return &website, nil
			}
		}

		if websitesResp.Meta.PerPage <= 0 {
			// A 2xx response whose body didn't decode into the expected
			// {data, meta} shape (a degraded/malformed payload from the API
			// or an edge in front of it) leaves Meta zero-valued rather than
			// erroring - json.Unmarshal doesn't fail on missing/mismatched
			// fields. Dividing by PerPage here unconditionally used to panic
			// the whole provider process (integer divide by zero) with no
			// recover() anywhere in the call stack, surfacing to Terraform
			// as "Plugin did not respond" for every other in-flight
			// resource. Treat it as the last usable page instead.
			break
		}
		totalPages := (websitesResp.Meta.Total + websitesResp.Meta.PerPage - 1) / websitesResp.Meta.PerPage
		if websitesResp.Meta.CurrentPage >= totalPages || len(websitesResp.Data) == 0 {
			break
		}
		page++
	}

	return nil, fmt.Errorf("website %s not found", domain)
}

// DeleteWebsite deletes a website from Hostinger
func (c *Client) DeleteWebsite(domain string) error {
	resp, err := c.makeRequest(http.MethodDelete, "/websites/"+domain, map[string]bool{
		"confirm": true,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

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

		var websitesResp WebsitesResponse
		if err := json.NewDecoder(resp.Body).Decode(&websitesResp); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		resp.Body.Close()

		allWebsites = append(allWebsites, websitesResp.Data...)

		if websitesResp.Meta.PerPage <= 0 {
			// See the matching guard in GetWebsite: a degraded/malformed 2xx
			// body leaves Meta.PerPage at its zero value instead of erroring,
			// and dividing by it below used to panic the whole process.
			break
		}
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

	var ordersResp OrdersResponse
	if err := json.NewDecoder(resp.Body).Decode(&ordersResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return ordersResp.Data, nil
}
