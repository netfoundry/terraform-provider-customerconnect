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

var _ datasource.DataSource = &connectorDataSource{}

func NewConnectorDataSource() datasource.DataSource {
	return &connectorDataSource{}
}

type connectorDataSource struct {
	client *customerConnectData
}

func (d *connectorDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_connector"
}

func (d *connectorDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single NetFoundry Connector by ID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Unique identifier of the connector to look up.",
			},
			"provider_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Provider this connector belongs to.",
			},
			"customer_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Customer this connector belongs to.",
			},
			"location_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Location this connector belongs to.",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Name of the connector.",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Description of the connector.",
			},
			"type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Type of the connector: DEVICE, GATEWAY, or SDK_EMBEDDED.",
			},
			"connector_model_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Connector model this connector inherits applications from.",
			},
			"enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the connector is enabled.",
			},
			"owner_identity_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identity that owns this connector.",
			},
			"created_by": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identity that created this connector.",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when this connector was created.",
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when this connector was last updated.",
			},
			"deleted_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when this connector was deleted.",
			},
			"deleted_by": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identity that deleted this connector.",
			},
			"deleted": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether this connector has been deleted.",
			},
			"ziti_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of the Ziti entity backing this connector.",
			},
			"ziti_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique name used for the Ziti identity.",
			},
			"online": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the connector is online in Ziti.",
			},
			"enrolled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the connector is enrolled in Ziti.",
			},
			"enrollment_jwt": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Enrollment JWT for the connector, present until the connector enrolls.",
			},
			"enrollment_expires_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Expiration timestamp of the current Ziti enrollment.",
			},
		},
	}
}

func (d *connectorDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *connectorDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state connectorModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/connectors/%s", d.client.apiBaseURL, state.ID.ValueString())

	respBody, _, err := doRequest(ctx, http.MethodGet, url, d.client.accessToken, nil)
	if err != nil {
		if errors.Is(err, errNotFound) {
			resp.Diagnostics.AddError("Connector not found",
				fmt.Sprintf("No connector found with id %q", state.ID.ValueString()))
			return
		}
		resp.Diagnostics.AddError("Read connector failed", err.Error())
		return
	}

	var conn apiConnector
	if err := json.Unmarshal(respBody, &conn); err != nil {
		resp.Diagnostics.AddError("Parse error", fmt.Sprintf("Failed to parse response: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, connectorFromAPI(conn))...)
}
