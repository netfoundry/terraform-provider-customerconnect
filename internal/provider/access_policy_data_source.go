package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// endpointNestedDSAttrs returns the computed nested attributes for sources/destinations
// in the data source schema.
func endpointNestedDSAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"connector_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Source/destination connector UUID.",
		},
		"location_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Source/destination location UUID.",
		},
		"connector_model_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Source/destination connector model UUID — matches any connector attached to the given model.",
		},
	}
}

var _ datasource.DataSource = &accessPolicyDataSource{}

func NewAccessPolicyDataSource() datasource.DataSource {
	return &accessPolicyDataSource{}
}

type accessPolicyDataSource struct {
	client *customerConnectData
}

func (d *accessPolicyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_access_policy"
}

func (d *accessPolicyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single NetFoundry AccessPolicy by ID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Unique identifier of the access policy to look up.",
			},
			"provider_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Provider this access policy belongs to.",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Display name of the access policy.",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Description of the access policy.",
			},
			"sources": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Source endpoints of the access policy.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: endpointNestedDSAttrs(),
				},
			},
			"destinations": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Destination endpoints of the access policy.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: endpointNestedDSAttrs(),
				},
			},
			"enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the access policy is currently projected onto the network.",
			},
			"created_by": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identity that created this access policy.",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when this access policy was created.",
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when this access policy was last updated.",
			},
			"deleted_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when this access policy was deleted.",
			},
			"deleted_by": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identity that deleted this access policy.",
			},
			"ziti_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Name of the underlying Ziti dial service policy.",
			},
		},
	}
}

func (d *accessPolicyDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	tflog.Debug(ctx, "Configuring access policy data source")
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

func (d *accessPolicyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state accessPolicyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading access policy data source", map[string]any{"id": state.ID.ValueString()})

	url := fmt.Sprintf("%s/access-policies/%s", d.client.apiBaseURL, state.ID.ValueString())

	respBody, _, err := doRequest(ctx, http.MethodGet, url, d.client.accessToken, nil)
	if err != nil {
		if errors.Is(err, errNotFound) {
			resp.Diagnostics.AddError("Access policy not found",
				fmt.Sprintf("No access policy found with id %q", state.ID.ValueString()))
			return
		}
		resp.Diagnostics.AddError("Read access policy failed", err.Error())
		return
	}

	var ap apiAccessPolicy
	if err := json.Unmarshal(respBody, &ap); err != nil {
		resp.Diagnostics.AddError("Parse error", fmt.Sprintf("Failed to parse response: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, accessPolicyFromAPI(ap))...)
}
