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

// connectorModelAddressDSAttrs returns the computed nested attributes for an
// address entry in the data source schema.
func connectorModelAddressDSAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"key": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Server-assigned, immutable identifier for this address. Connector overrides bind to this key.",
		},
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
			MarkdownDescription: "Whether the host config forwards the intercepted address.",
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
			MarkdownDescription: "Whether the host config forwards the intercepted port.",
		},
		"target_port": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Target port for the host config.",
		},
		"allowed_ports": schema.ListAttribute{
			ElementType:         types.StringType,
			Computed:            true,
			MarkdownDescription: "Allowed forward ports or port ranges.",
		},
		"overridable_fields": schema.ListAttribute{
			ElementType:         types.StringType,
			Computed:            true,
			MarkdownDescription: "Field names on a connector's address override that may be set for this address.",
		},
		"required_fields": schema.ListAttribute{
			ElementType:         types.StringType,
			Computed:            true,
			MarkdownDescription: "Field names whose value this model does not pin down, which the connector must supply via override before the address can materialize.",
		},
	}
}

// connectorModelApplicationDSAttrs returns the computed nested attributes for an
// application entry in the data source schema.
func connectorModelApplicationDSAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"key": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Server-assigned, immutable identifier for this application. Connector overrides bind to this key.",
		},
		"name": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Default name of the application.",
		},
		"type": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Type of the application.",
		},
		"protocol": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Protocol (`TCP`, `UDP`, or `TCP_UDP`).",
		},
		"addresses": schema.ListNestedAttribute{
			Computed:            true,
			MarkdownDescription: "Service address mappings.",
			NestedObject: schema.NestedAttributeObject{
				Attributes: connectorModelAddressDSAttrs(),
			},
		},
		"overridable_fields": schema.ListAttribute{
			ElementType:         types.StringType,
			Computed:            true,
			MarkdownDescription: "Field names on a connector's applications that may be set for this application.",
		},
		"required_fields": schema.ListAttribute{
			ElementType:         types.StringType,
			Computed:            true,
			MarkdownDescription: "Field names whose value this model does not pin down, which the connector must supply via override before the application can be materialized.",
		},
	}
}

// connectorModelCountsDSAttrs returns the computed attributes for the counts
// nested object in the data source schema.
func connectorModelCountsDSAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"applications": schema.Int64Attribute{
			Computed:            true,
			MarkdownDescription: "Number of applications inherited from this connector model.",
		},
		"source_access_policies": schema.Int64Attribute{
			Computed:            true,
			MarkdownDescription: "Number of access policies that include this connector model as a source.",
		},
		"destination_access_policies": schema.Int64Attribute{
			Computed:            true,
			MarkdownDescription: "Number of access policies that include this connector model as a destination.",
		},
		"source_connections": schema.Int64Attribute{
			Computed:            true,
			MarkdownDescription: "Deprecated alias of source_access_policies.",
		},
		"destination_connections": schema.Int64Attribute{
			Computed:            true,
			MarkdownDescription: "Deprecated alias of destination_access_policies.",
		},
	}
}

var _ datasource.DataSource = &connectorModelDataSource{}

func NewConnectorModelDataSource() datasource.DataSource {
	return &connectorModelDataSource{}
}

type connectorModelDataSource struct {
	client *customerConnectData
}

func (d *connectorModelDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_connector_model"
}

func (d *connectorModelDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single NetFoundry Connector Model by ID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Unique identifier of the connector model to look up.",
			},
			"provider_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Provider this connector model belongs to.",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Name of the connector model.",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Description of the connector model.",
			},
			"type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Type of the connector model: DEVICE, GATEWAY, or SDK_EMBEDDED.",
			},
			"applications": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Applications defined in this connector model.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: connectorModelApplicationDSAttrs(),
				},
			},
			"owner_identity_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identity that owns this connector model.",
			},
			"created_by": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identity that created this connector model.",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when this connector model was created.",
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when this connector model was last updated.",
			},
			"deleted_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when this connector model was deleted.",
			},
			"deleted_by": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identity that deleted this connector model.",
			},
			"deleted": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether this connector model has been deleted.",
			},
			"counts": schema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Sub-resource counts for this connector model.",
				Attributes:          connectorModelCountsDSAttrs(),
			},
		},
	}
}

func (d *connectorModelDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *connectorModelDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state connectorModelModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/connector-models/%s", d.client.apiBaseURL, state.ID.ValueString())

	respBody, _, err := doRequest(ctx, http.MethodGet, url, d.client.accessToken, nil)
	if err != nil {
		if errors.Is(err, errNotFound) {
			resp.Diagnostics.AddError("Connector model not found",
				fmt.Sprintf("No connector model found with id %q", state.ID.ValueString()))
			return
		}
		resp.Diagnostics.AddError("Read connector model failed", err.Error())
		return
	}

	var cm apiConnectorModel
	if err := json.Unmarshal(respBody, &cm); err != nil {
		resp.Diagnostics.AddError("Parse error", fmt.Sprintf("Failed to parse response: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, connectorModelFromAPI(cm))...)
}
