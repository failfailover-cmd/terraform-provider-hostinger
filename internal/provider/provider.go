package provider

import (
	"context"
	"os"
	"time"

	"github.com/failfailover-cmd/terraform-provider-hostinger/internal/provider/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ provider.Provider = &hostingerProvider{}
)

// New is a helper function to simplify provider server and testing implementation.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &hostingerProvider{
			version: version,
		}
	}
}

// hostingerProvider is the provider implementation.
type hostingerProvider struct {
	version string
}

// hostingerProviderModel maps provider schema data to a Go type.
type hostingerProviderModel struct {
	APIToken             types.String `tfsdk:"api_token"`
	MaxRetries           types.Int64  `tfsdk:"max_retries"`
	BaseBackoffMS        types.Int64  `tfsdk:"base_backoff_ms"`
	MaxBackoffMS         types.Int64  `tfsdk:"max_backoff_ms"`
	MinRequestIntervalMS types.Int64  `tfsdk:"min_request_interval_ms"`
}

// Metadata returns the provider type name.
func (p *hostingerProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "hostinger"
	resp.Version = p.version
}

// Schema defines the provider-level schema for configuration data.
func (p *hostingerProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Interact with Hostinger Hosting API.",
		Attributes: map[string]schema.Attribute{
			"api_token": schema.StringAttribute{
				Description: "API token for Hostinger API. May also be provided via HOSTINGER_API_TOKEN environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
			"max_retries": schema.Int64Attribute{
				Description: "Maximum retry attempts for retryable API errors (429/1015/5xx). Default: 7.",
				Optional:    true,
			},
			"base_backoff_ms": schema.Int64Attribute{
				Description: "Base backoff in milliseconds for retries. Default: 2000.",
				Optional:    true,
			},
			"max_backoff_ms": schema.Int64Attribute{
				Description: "Maximum backoff in milliseconds for retries. Default: 60000.",
				Optional:    true,
			},
			"min_request_interval_ms": schema.Int64Attribute{
				Description: "Minimum delay between API requests in milliseconds. Default: 1200.",
				Optional:    true,
			},
		},
	}
}

// Configure prepares a Hostinger API client for data sources and resources.
func (p *hostingerProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	// Retrieve provider data from configuration
	var config hostingerProviderModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If practitioner provided a configuration value for any of the
	// attributes, it must be a known value.

	if config.APIToken.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_token"),
			"Unknown Hostinger API Token",
			"The provider cannot create the Hostinger API client as there is an unknown configuration value for the Hostinger API token. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the HOSTINGER_API_TOKEN environment variable.",
		)
	}
	if config.MaxRetries.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("max_retries"),
			"Unknown max_retries",
			"The provider cannot initialize with an unknown max_retries value.",
		)
	}
	if config.BaseBackoffMS.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("base_backoff_ms"),
			"Unknown base_backoff_ms",
			"The provider cannot initialize with an unknown base_backoff_ms value.",
		)
	}
	if config.MaxBackoffMS.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("max_backoff_ms"),
			"Unknown max_backoff_ms",
			"The provider cannot initialize with an unknown max_backoff_ms value.",
		)
	}
	if config.MinRequestIntervalMS.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("min_request_interval_ms"),
			"Unknown min_request_interval_ms",
			"The provider cannot initialize with an unknown min_request_interval_ms value.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Default values to environment variables, but override
	// with Terraform configuration value if set.

	apiToken := os.Getenv("HOSTINGER_API_TOKEN")

	if !config.APIToken.IsNull() {
		apiToken = config.APIToken.ValueString()
	}

	// If any of the expected configurations are missing, return
	// errors with provider-specific guidance.

	if apiToken == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_token"),
			"Missing Hostinger API Token",
			"The provider cannot create the Hostinger API client as there is a missing or empty value for the Hostinger API token. "+
				"Set the api_token value in the configuration or use the HOSTINGER_API_TOKEN environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	maxRetries := int64(client.MaxRetries)
	if !config.MaxRetries.IsNull() {
		maxRetries = config.MaxRetries.ValueInt64()
	}
	baseBackoffMS := int64(client.BaseBackoff / time.Millisecond)
	if !config.BaseBackoffMS.IsNull() {
		baseBackoffMS = config.BaseBackoffMS.ValueInt64()
	}
	maxBackoffMS := int64(client.MaxBackoff / time.Millisecond)
	if !config.MaxBackoffMS.IsNull() {
		maxBackoffMS = config.MaxBackoffMS.ValueInt64()
	}
	minReqIntervalMS := int64(client.DefaultMinRequestInterval / time.Millisecond)
	if !config.MinRequestIntervalMS.IsNull() {
		minReqIntervalMS = config.MinRequestIntervalMS.ValueInt64()
	}

	// Create a new Hostinger client using the configuration values.
	hostingerClient := client.NewClientWithConfig(client.Config{
		APIToken:           apiToken,
		MaxRetries:         int(maxRetries),
		BaseBackoff:        time.Duration(baseBackoffMS) * time.Millisecond,
		MaxBackoff:         time.Duration(maxBackoffMS) * time.Millisecond,
		MinRequestInterval: time.Duration(minReqIntervalMS) * time.Millisecond,
	})

	// Make the Hostinger client available during DataSource and Resource
	// type Configure methods.
	resp.DataSourceData = hostingerClient
	resp.ResourceData = hostingerClient
}

// DataSources defines the data sources implemented in the provider.
func (p *hostingerProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewOrdersDataSource,
		NewWebsitesDataSource,
	}
}

// Resources defines the resources implemented in the provider.
func (p *hostingerProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewWebsiteResource,
	}
}
