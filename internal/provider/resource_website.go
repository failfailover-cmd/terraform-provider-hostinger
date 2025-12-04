package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"gitlab.com/a4765/infra/devops/terraform_providers/hostinger/internal/provider/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &websiteResource{}
var _ resource.ResourceWithImportState = &websiteResource{}

func NewWebsiteResource() resource.Resource {
	return &websiteResource{}
}

// websiteResource defines the resource implementation.
type websiteResource struct {
	client *client.Client
}

// websiteResourceModel describes the resource data model.
type websiteResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Domain         types.String `tfsdk:"domain"`
	OrderID        types.Int64  `tfsdk:"order_id"`
	DatacenterCode types.String `tfsdk:"datacenter_code"`
}

func (r *websiteResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_website"
}

func (r *websiteResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Hostinger Website resource. Manages a website on a Hostinger hosting plan.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Website ID (same as domain name)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"domain": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Domain name for the website (without www).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"order_id": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "ID of the hosting order/plan where the website will be created.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"datacenter_code": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Datacenter code. Required only when creating the first website on a new hosting plan.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *websiteResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *websiteResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data websiteResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	domain := data.Domain.ValueString()
	orderID := int(data.OrderID.ValueInt64())
	datacenterCode := data.DatacenterCode.ValueString()

	tflog.Info(ctx, fmt.Sprintf("Creating website %s on order %d", domain, orderID))

	err := r.client.CreateWebsite(domain, orderID, datacenterCode)
	if err != nil {
		resp.Diagnostics.AddError(
			"Client Error",
			fmt.Sprintf("Unable to create website, got error: %s", err),
		)
		return
	}

	// Write logs using the tflog package
	// This provider writes data to the Terraform log file
	// To view the logs, set the TF_LOG environment variable to DEBUG
	tflog.Trace(ctx, "created a website")

	// Save data into Terraform state
	data.ID = types.StringValue(domain)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *websiteResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data websiteResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	domain := data.Domain.ValueString()

	tflog.Info(ctx, fmt.Sprintf("Reading website %s", domain))

	website, err := r.client.GetWebsite(domain)
	if err != nil {
		// If website is not found, remove from state
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Client Error",
			fmt.Sprintf("Unable to read website, got error: %s", err),
		)
		return
	}

	// Update state with refreshed data
	data.ID = types.StringValue(website.Domain)
	data.Domain = types.StringValue(website.Domain)
	data.OrderID = types.Int64Value(int64(website.OrderID))
	// Note: DatacenterCode is not returned by the API in the website object usually,
	// so we keep the value from the state if it exists.

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *websiteResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Hostinger API does not support updating website parameters like domain or order_id in place.
	// Changing these requires recreation, which is handled by PlanModifiers (RequiresReplace).
	// So this method shouldn't be called for those attributes.
	resp.Diagnostics.AddError(
		"Update Not Supported",
		"Hostinger websites cannot be updated in-place. Any change forces recreation.",
	)
}

func (r *websiteResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data websiteResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	domain := data.Domain.ValueString()

	tflog.Info(ctx, fmt.Sprintf("Deleting website %s", domain))

	err := r.client.DeleteWebsite(domain)
	if err != nil {
		resp.Diagnostics.AddError(
			"Client Error",
			fmt.Sprintf("Unable to delete website, got error: %s", err),
		)
		return
	}
}

func (r *websiteResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("domain"), req, resp)
}
