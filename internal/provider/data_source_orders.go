package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/yourusername/terraform-provider-hostinger/internal/provider/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &ordersDataSource{}

func NewOrdersDataSource() datasource.DataSource {
	return &ordersDataSource{}
}

// ordersDataSource defines the data source implementation.
type ordersDataSource struct {
	client *client.Client
}

// ordersDataSourceModel describes the data source data model.
type ordersDataSourceModel struct {
	Orders []orderModel `tfsdk:"orders"`
}

type orderModel struct {
	ID             types.Int64  `tfsdk:"id"`
	ClientID       types.Int64  `tfsdk:"client_id"`
	SubscriptionID types.String `tfsdk:"subscription_id"`
	CreatedAt      types.String `tfsdk:"created_at"`
	PlanName       types.String `tfsdk:"plan_name"`
	Status         types.String `tfsdk:"status"`
}

func (d *ordersDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_orders"
}

func (d *ordersDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Orders data source. Lists all hosting orders/plans.",

		Attributes: map[string]schema.Attribute{
			"orders": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.Int64Attribute{
							Computed: true,
						},
						"client_id": schema.Int64Attribute{
							Computed: true,
						},
						"subscription_id": schema.StringAttribute{
							Computed: true,
						},
						"created_at": schema.StringAttribute{
							Computed: true,
						},
						"plan_name": schema.StringAttribute{
							Computed: true,
						},
						"status": schema.StringAttribute{
							Computed: true,
						},
					},
				},
			},
		},
	}
}

func (d *ordersDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ordersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ordersDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	orders, err := d.client.ListOrders()
	if err != nil {
		resp.Diagnostics.AddError(
			"Client Error",
			fmt.Sprintf("Unable to read orders, got error: %s", err),
		)
		return
	}

	for _, order := range orders {
		orderState := orderModel{
			ID:             types.Int64Value(int64(order.ID)),
			ClientID:       types.Int64Value(int64(order.ClientID)),
			SubscriptionID: types.StringValue(order.SubscriptionID),
			CreatedAt:      types.StringValue(order.CreatedAt),
			PlanName:       types.StringValue(order.Plan.Name),
			Status:         types.StringValue(order.Status),
		}

		data.Orders = append(data.Orders, orderState)
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
