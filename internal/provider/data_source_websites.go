package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/failfailover-cmd/terraform-provider-hostinger/internal/provider/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &websitesDataSource{}

func NewWebsitesDataSource() datasource.DataSource {
	return &websitesDataSource{}
}

// websitesDataSource defines the data source implementation.
type websitesDataSource struct {
	client *client.Client
}

// websitesDataSourceModel describes the data source data model.
type websitesDataSourceModel struct {
	Websites []websiteModel `tfsdk:"websites"`
}

type websiteModel struct {
	Domain        types.String `tfsdk:"domain"`
	OrderID       types.Int64  `tfsdk:"order_id"`
	VhostType     types.String `tfsdk:"vhost_type"`
	IsEnabled     types.Bool   `tfsdk:"is_enabled"`
	Username      types.String `tfsdk:"username"`
	ClientID      types.Int64  `tfsdk:"client_id"`
	CreatedAt     types.String `tfsdk:"created_at"`
	RootDirectory types.String `tfsdk:"root_directory"`
}

func (d *websitesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_websites"
}

func (d *websitesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Websites data source. Lists all websites.",

		Attributes: map[string]schema.Attribute{
			"websites": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"domain": schema.StringAttribute{
							Computed: true,
						},
						"order_id": schema.Int64Attribute{
							Computed: true,
						},
						"vhost_type": schema.StringAttribute{
							Computed: true,
						},
						"is_enabled": schema.BoolAttribute{
							Computed: true,
						},
						"username": schema.StringAttribute{
							Computed: true,
						},
						"client_id": schema.Int64Attribute{
							Computed: true,
						},
						"created_at": schema.StringAttribute{
							Computed: true,
						},
						"root_directory": schema.StringAttribute{
							Computed: true,
						},
					},
				},
			},
		},
	}
}

func (d *websitesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	d.client = client
}

func (d *websitesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	defer recoverIntoDiagnostics(&resp.Diagnostics)

	var data websitesDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	websites, err := d.client.ListWebsites()
	if err != nil {
		resp.Diagnostics.AddError(
			"Client Error",
			fmt.Sprintf("Unable to read websites, got error: %s", err),
		)
		return
	}

	for _, website := range websites {
		websiteState := websiteModel{
			Domain:        types.StringValue(website.Domain),
			OrderID:       types.Int64Value(int64(website.OrderID)),
			VhostType:     types.StringValue(website.VhostType),
			IsEnabled:     types.BoolValue(website.IsEnabled),
			Username:      types.StringValue(website.Username),
			ClientID:      types.Int64Value(int64(website.ClientID)),
			CreatedAt:     types.StringValue(website.CreatedAt),
			RootDirectory: types.StringValue(website.RootDirectory),
		}

		data.Websites = append(data.Websites, websiteState)
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
