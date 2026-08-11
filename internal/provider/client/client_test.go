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
