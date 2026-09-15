package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/failfailover-cmd/terraform-provider-hostinger/internal/provider/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestWebsiteReadOnlyRemovesStateForConfirmedAbsence(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		status  int
		body    string
		missing bool
	}{
		{"empty inventory", http.StatusOK, `{"data":[],"meta":{"current_page":1,"per_page":100,"total":0}}`, true},
		{"missing metadata", http.StatusOK, `{"data":[]}`, false},
		{"endpoint not found", http.StatusNotFound, `{"message":"endpoint not found"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			c := client.NewClient("test-token")
			c.BaseURL = server.URL
			c.MinRequestInterval = 0
			r := &websiteResource{client: c}
			ctx := context.Background()
			var schema resource.SchemaResponse
			r.Schema(ctx, resource.SchemaRequest{}, &schema)
			state := tfsdk.State{Schema: schema.Schema}
			diagnostics := state.Set(ctx, &websiteResourceModel{
				ID: types.StringValue("site.example"), Domain: types.StringValue("site.example"),
				OrderID: types.Int64Value(1), DatacenterCode: types.StringNull(),
			})
			if diagnostics.HasError() {
				t.Fatal(diagnostics)
			}
			resp := resource.ReadResponse{State: state}
			r.Read(ctx, resource.ReadRequest{State: state}, &resp)
			if tc.missing {
				if resp.Diagnostics.HasError() || !resp.State.Raw.IsNull() {
					t.Fatalf("confirmed absence did not remove state: %v", resp.Diagnostics)
				}
			} else if !resp.Diagnostics.HasError() || !resp.State.Raw.Equal(state.Raw) {
				t.Fatalf("API error must preserve state and return a diagnostic: %v", resp.Diagnostics)
			}
		})
	}
}
