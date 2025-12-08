package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"gitlab.com/a4765/infra/devops/terraform_providers/hostinger/internal/provider/client"
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
	APIToken types.String `tfsdk:"api_token"`
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

	// Create a new Hostinger client using the configuration values
	hostingerClient := client.NewClient(apiToken)

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
