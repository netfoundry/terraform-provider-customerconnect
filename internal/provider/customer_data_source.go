package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

// customerCountsDSAttrs returns the computed attributes for the counts
// nested object in the data source schema.
func customerCountsDSAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"locations": schema.Int64Attribute{
			Computed:            true,
			MarkdownDescription: "Number of locations under this customer.",
		},
		"connectors": schema.Int64Attribute{
			Computed:            true,
			MarkdownDescription: "Number of connectors under this customer.",
		},
	}
}

var _ datasource.DataSource = &customerDataSource{}

func NewCustomerDataSource() datasource.DataSource {
	return &customerDataSource{}
}

type customerDataSource struct {
	client *customerConnectData
}

func (d *customerDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_customer"
}

func (d *customerDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single NetFoundry Customer by ID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Unique identifier of the customer to look up.",
			},
			"provider_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Provider this customer belongs to.",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Name of the customer.",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Description of the customer.",
			},
			"enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the customer is enabled.",
			},
			"owner_identity_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identity that owns this customer.",
			},
			"created_by": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identity that created this customer.",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when this customer was created.",
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when this customer was last updated.",
			},
			"deleted_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when this customer was deleted.",
			},
			"deleted_by": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identity that deleted this customer.",
			},
			"deleted": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether this customer has been deleted.",
			},
			"counts": schema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Sub-resource counts for this customer.",
				Attributes:          customerCountsDSAttrs(),
			},
		},
	}
}

func (d *customerDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data, ok := req.ProviderData.(*customerConnectData)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type",
			fmt.Sprintf("Expected *customerConnectData, got %T", req.ProviderData))
		return
	}
	d.client = data
}

func (d *customerDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state customerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/customers/%s", d.client.apiBaseURL, state.ID.ValueString())

	respBody, _, err := doRequest(ctx, http.MethodGet, url, d.client.accessToken, nil)
	if err != nil {
		if errors.Is(err, errNotFound) {
			resp.Diagnostics.AddError("Customer not found",
				fmt.Sprintf("No customer found with id %q", state.ID.ValueString()))
			return
		}
		resp.Diagnostics.AddError("Read customer failed", err.Error())
		return
	}

	var c apiCustomer
	if err := json.Unmarshal(respBody, &c); err != nil {
		resp.Diagnostics.AddError("Parse error", fmt.Sprintf("Failed to parse response: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, customerFromAPI(c))...)
}
