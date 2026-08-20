package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

// A rate-limiting edge in front of the real API can return an arbitrarily
// large Retry-After (minutes to hours). Honoring it verbatim turned a single
// resource's Read into a sleep that outlasts CI job timeouts with no visible
// network activity - indistinguishable from a genuine hang. This reproduces
// that on the pre-fix retryAfterOrBackoff (a bare time.Duration(seconds)
// return) and confirms the fixed version caps to MaxBackoff.
func TestRetryAfterIsCappedToMaxBackoff(t *testing.T) {
	t.Parallel()

	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			w.Header().Set("Retry-After", "3600")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"message":"rate limited"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[],"meta":{"current_page":1,"per_page":10,"total":0}}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.BaseURL = server.URL
	client.MinRequestInterval = 0
	client.MaxBackoff = 200 * time.Millisecond

	start := time.Now()
	if _, err := client.ListWebsites(); err != nil {
		t.Fatalf("ListWebsites() error = %v", err)
	}
	elapsed := time.Since(start)

	if elapsed > time.Second {
		t.Fatalf("ListWebsites() took %s, want capped to ~MaxBackoff (%s), not the full Retry-After", elapsed, client.MaxBackoff)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2 (one 429, one success)", requests)
	}
}

func TestCapDuration(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		d    time.Duration
		max  time.Duration
		want time.Duration
	}{
		{"under max", 5 * time.Second, 60 * time.Second, 5 * time.Second},
		{"over max", 3600 * time.Second, 60 * time.Second, 60 * time.Second},
		{"equal to max", 60 * time.Second, 60 * time.Second, 60 * time.Second},
		{"zero max means unbounded", 3600 * time.Second, 0, 3600 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := capDuration(tc.d, tc.max); got != tc.want {
				t.Errorf("capDuration(%s, %s) = %s, want %s", tc.d, tc.max, got, tc.want)
			}
		})
	}
}
