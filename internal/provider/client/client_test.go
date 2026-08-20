package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeleteWebsiteSendsConfirmation(t *testing.T) {
	t.Parallel()

	const domain = "example.com"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want %q", r.Method, http.MethodDelete)
		}
		if r.URL.Path != "/websites/"+domain {
			t.Errorf("path = %q, want %q", r.URL.Path, "/websites/"+domain)
		}

		var body struct {
			Confirm bool `json:"confirm"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if !body.Confirm {
			t.Error("confirm = false, want true")
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.BaseURL = server.URL
	client.MinRequestInterval = 0

	if err := client.DeleteWebsite(domain); err != nil {
		t.Fatalf("DeleteWebsite() error = %v", err)
	}
}

// A 2xx response whose body doesn't match the expected {data, meta} shape
// (a degraded/malformed payload, or an edge in front of the real API
// returning something unexpected while still answering 200) leaves
// Meta.PerPage at its zero value - json.Unmarshal doesn't error on
// missing/mismatched fields. GetWebsite/ListWebsites used to divide by
// PerPage unconditionally, panicking the whole provider process with no
// recover() anywhere in the call stack (see the client.go comments this
// commit adds). These tests would panic the test binary itself pre-fix.
func degradedWebsitesServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[{"domain":"other.example"}]}`))
	}))
}

func TestGetWebsiteDoesNotPanicOnDegradedResponse(t *testing.T) {
	t.Parallel()

	server := degradedWebsitesServer(t)
	defer server.Close()

	client := NewClient("test-token")
	client.BaseURL = server.URL
	client.MinRequestInterval = 0

	_, err := client.GetWebsite("not-in-the-degraded-page.example")
	if err == nil {
		t.Fatal("GetWebsite() error = nil, want a not-found error")
	}
}

func TestListWebsitesDoesNotPanicOnDegradedResponse(t *testing.T) {
	t.Parallel()

	server := degradedWebsitesServer(t)
	defer server.Close()

	client := NewClient("test-token")
	client.BaseURL = server.URL
	client.MinRequestInterval = 0

	websites, err := client.ListWebsites()
	if err != nil {
		t.Fatalf("ListWebsites() error = %v", err)
	}
	if len(websites) != 1 || websites[0].Domain != "other.example" {
		t.Fatalf("ListWebsites() = %+v, want the single degraded-page entry", websites)
	}
}
