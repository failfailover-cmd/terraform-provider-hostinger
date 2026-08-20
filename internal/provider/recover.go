package provider

import (
	"fmt"
	"runtime/debug"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// recoverIntoDiagnostics converts a panic in the calling resource/data-source
// method into a Terraform diagnostic error instead of letting it crash the
// whole provider process. An unrecovered panic in any one RPC (e.g. the
// GetWebsite/ListWebsites divide-by-zero on a degraded API response, see
// client.go) kills the entire Go process - Go does not isolate goroutine
// panics - which Terraform core then reports as "Plugin did not respond"
// for every other in-flight resource, not just the one that panicked.
//
// Call as `defer recoverIntoDiagnostics(&resp.Diagnostics)` at the top of
// any method whose resp has a Diagnostics field.
func recoverIntoDiagnostics(diags *diag.Diagnostics) {
	if p := recover(); p != nil {
		diags.AddError(
			"Panic Recovered",
			fmt.Sprintf("The provider encountered an unexpected panic: %v\n\n%s", p, debug.Stack()),
		)
	}
}
