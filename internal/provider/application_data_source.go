package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// applicationAddressDSAttrs returns the computed nested attributes for an
// address entry in the data source schema.
func applicationAddressDSAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"listen_address": schema.ListAttribute{
			ElementType:         types.StringType,
			Computed:            true,
			MarkdownDescription: "Listen addresses (hosts) for the intercept config.",
		},
		"listen_port": schema.ListAttribute{
			ElementType:         types.StringType,
			Computed:            true,
			MarkdownDescription: "Listen ports or port ranges for the intercept config.",
		},
		"forward_address": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Forward the listening address.",
		},
		"target_address": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Target address for the host config.",
		},
		"allowed_addresses": schema.ListAttribute{
			ElementType:         types.StringType,
			Computed:            true,
			MarkdownDescription: "Allowed forward addresses (IPs, CIDR blocks, or hostnames).",
		},
		"forward_port": schema.BoolAttribute{
			Computed:            true,
			MarkdownDescription: "Forward the listening port.",
		},
		"target_port": schema.Int64Attribute{
			Computed:            true,
			MarkdownDescription: "Target port for the host config.",
		},
		"allowed_ports": schema.ListAttribute{
			ElementType:         types.StringType,
			Computed:            true,
			MarkdownDescription: "Allowed forward ports or port ranges.",
		},
		"ziti_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Ziti service id of the service backing this address.",
		},
	}
}

var _ datasource.DataSource = &applicationDataSource{}

func NewApplicationDataSource() datasource.DataSource {
	return &applicationDataSource{}
}

type applicationDataSource struct {
	client *customerConnectData
}

func (d *applicationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application"
}

func (d *applicationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single NetFoundry Application by ID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Unique identifier of the application to look up.",
			},
			"connector_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Connector this application belongs to.",
			},
			"provider_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Provider this application belongs to.",
			},
			"connector_model_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "If set, this application is inherited from the referenced connector model and cannot be modified directly.",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Display name of the application.",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Description of the application.",
			},
			"type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Type of the application.",
			},
			"protocol": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Protocol (`TCP`, `UDP`, or `TCP_UDP`).",
			},
			"enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "When false, the application is not materialized into a Ziti service.",
			},
			"ziti_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Ziti service id of the primary service backing this application. Null while the application is disabled.",
			},
			"addresses": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Service address mappings.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: applicationAddressDSAttrs(),
				},
			},
			"created_by": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identity that created this application.",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when this application was created.",
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when this application was last updated.",
			},
			"deleted_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when this application was deleted.",
			},
			"deleted_by": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identity that deleted this application.",
			},
			"ziti_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique name used for the primary Ziti service, formatted as \"<providerId>|<connectorId>|<name>\".",
			},
		},
	}
}

func (d *applicationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *applicationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state applicationModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/connectors/%s/applications/%s", d.client.apiBaseURL, state.ConnectorID.ValueString(), state.ID.ValueString())

	respBody, _, err := doRequest(ctx, http.MethodGet, url, d.client.accessToken, nil)
	if err != nil {
		if errors.Is(err, errNotFound) {
			resp.Diagnostics.AddError("Application not found",
				fmt.Sprintf("No application found with id %q on connector %q", state.ID.ValueString(), state.ConnectorID.ValueString()))
			return
		}
		resp.Diagnostics.AddError("Read application failed", err.Error())
		return
	}

	var app apiApplication
	if err := json.Unmarshal(respBody, &app); err != nil {
		resp.Diagnostics.AddError("Parse error", fmt.Sprintf("Failed to parse response: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, applicationFromAPI(app))...)
}
