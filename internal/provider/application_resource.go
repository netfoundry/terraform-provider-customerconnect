package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &applicationResource{}
var _ resource.ResourceWithImportState = &applicationResource{}

func NewApplicationResource() resource.Resource {
	return &applicationResource{}
}

type applicationResource struct {
	client *customerConnectData
}

// applicationAddressModel is shared by both the resource and the data source.
type applicationAddressModel struct {
	ListenAddress    []string     `tfsdk:"listen_address"`
	ListenPort       []string     `tfsdk:"listen_port"`
	ForwardAddress   types.Bool   `tfsdk:"forward_address"`
	TargetAddress    types.String `tfsdk:"target_address"`
	AllowedAddresses []string     `tfsdk:"allowed_addresses"`
	ForwardPort      types.Bool   `tfsdk:"forward_port"`
	TargetPort       types.Int64  `tfsdk:"target_port"`
	AllowedPorts     []string     `tfsdk:"allowed_ports"`
	ZitiID           types.String `tfsdk:"ziti_id"`
}

// applicationModel is shared by both the resource and the data source.
type applicationModel struct {
	ID               types.String               `tfsdk:"id"`
	ProviderID       types.String               `tfsdk:"provider_id"`
	ConnectorID      types.String               `tfsdk:"connector_id"`
	ConnectorModelID types.String               `tfsdk:"connector_model_id"`
	Name             types.String               `tfsdk:"name"`
	Description      types.String               `tfsdk:"description"`
	Type             types.String               `tfsdk:"type"`
	Protocol         types.String               `tfsdk:"protocol"`
	Enabled          types.Bool                 `tfsdk:"enabled"`
	ZitiID           types.String               `tfsdk:"ziti_id"`
	Addresses        []applicationAddressModel  `tfsdk:"addresses"`
	CreatedBy        types.String               `tfsdk:"created_by"`
	CreatedAt        types.String               `tfsdk:"created_at"`
	UpdatedAt        types.String               `tfsdk:"updated_at"`
	DeletedAt        types.String               `tfsdk:"deleted_at"`
	DeletedBy        types.String               `tfsdk:"deleted_by"`
	ZitiName         types.String               `tfsdk:"ziti_name"`
}

// apiServiceAddress mirrors the ServiceAddress JSON returned by the API.
type apiServiceAddress struct {
	ListenAddress    []string `json:"listenAddress"`
	ListenPort       []string `json:"listenPort"`
	ForwardAddress   bool     `json:"forwardAddress"`
	TargetAddress    string   `json:"targetAddress"`
	AllowedAddresses []string `json:"allowedAddresses"`
	ForwardPort      bool     `json:"forwardPort"`
	TargetPort       *int64   `json:"targetPort"`
	AllowedPorts     []string `json:"allowedPorts"`
	ZitiID           string   `json:"zitiId"`
}

// apiApplication mirrors the Application JSON returned by the API.
type apiApplication struct {
	ID               string              `json:"id"`
	ProviderID       string              `json:"providerId"`
	ConnectorID      string              `json:"connectorId"`
	ConnectorModelID string              `json:"connectorModelId"`
	Name             string              `json:"name"`
	Description      string              `json:"description"`
	Type             string              `json:"type"`
	Protocol         string              `json:"protocol"`
	Enabled          bool                `json:"enabled"`
	ZitiID           string              `json:"zitiId"`
	Addresses        []apiServiceAddress `json:"addresses"`
	CreatedBy        string              `json:"createdBy"`
	CreatedAt        string              `json:"createdAt"`
	UpdatedAt        string              `json:"updatedAt"`
	DeletedAt        string              `json:"deletedAt"`
	DeletedBy        string              `json:"deletedBy"`
	ZitiName         string              `json:"zitiName"`
}

// createServiceAddressPayload is the wire format for creating/replacing a service address.
type createServiceAddressPayload struct {
	ListenAddress    []string `json:"listenAddress"`
	ListenPort       []string `json:"listenPort"`
	ForwardAddress   *bool    `json:"forwardAddress,omitempty"`
	TargetAddress    string   `json:"targetAddress,omitempty"`
	AllowedAddresses []string `json:"allowedAddresses,omitempty"`
	ForwardPort      *bool    `json:"forwardPort,omitempty"`
	TargetPort       *int64   `json:"targetPort,omitempty"`
	AllowedPorts     []string `json:"allowedPorts,omitempty"`
}

type createApplicationPayload struct {
	Name        string                        `json:"name"`
	Description string                        `json:"description,omitempty"`
	Type        string                        `json:"type,omitempty"`
	Protocol    string                        `json:"protocol,omitempty"`
	Enabled     *bool                         `json:"enabled,omitempty"`
	Addresses   []createServiceAddressPayload `json:"addresses,omitempty"`
}

type updateApplicationPayload struct {
	Name        string                        `json:"name,omitempty"`
	Description string                        `json:"description,omitempty"`
	Type        string                        `json:"type,omitempty"`
	Protocol    string                        `json:"protocol,omitempty"`
	Enabled     *bool                         `json:"enabled,omitempty"`
	Addresses   []createServiceAddressPayload `json:"addresses,omitempty"`
}

// addressToAPI converts an address model to the API wire format.
func addressToAPI(m applicationAddressModel) createServiceAddressPayload {
	p := createServiceAddressPayload{
		ListenAddress: m.ListenAddress,
		ListenPort:    m.ListenPort,
	}
	if !m.ForwardAddress.IsNull() && !m.ForwardAddress.IsUnknown() {
		v := m.ForwardAddress.ValueBool()
		p.ForwardAddress = &v
	}
	if !m.TargetAddress.IsNull() && !m.TargetAddress.IsUnknown() {
		p.TargetAddress = m.TargetAddress.ValueString()
	}
	if len(m.AllowedAddresses) > 0 {
		p.AllowedAddresses = m.AllowedAddresses
	}
	if !m.ForwardPort.IsNull() && !m.ForwardPort.IsUnknown() {
		v := m.ForwardPort.ValueBool()
		p.ForwardPort = &v
	}
	if !m.TargetPort.IsNull() && !m.TargetPort.IsUnknown() {
		v := m.TargetPort.ValueInt64()
		p.TargetPort = &v
	}
	if len(m.AllowedPorts) > 0 {
		p.AllowedPorts = m.AllowedPorts
	}
	return p
}

// addressesToAPI converts a slice of address models to the API wire format.
func addressesToAPI(addresses []applicationAddressModel) []createServiceAddressPayload {
	result := make([]createServiceAddressPayload, len(addresses))
	for i, a := range addresses {
		result[i] = addressToAPI(a)
	}
	return result
}

// addressFromAPI maps an API response to the shared applicationAddressModel.
func addressFromAPI(api apiServiceAddress) applicationAddressModel {
	m := applicationAddressModel{
		ListenAddress:    api.ListenAddress,
		ListenPort:       api.ListenPort,
		ForwardAddress:   types.BoolValue(api.ForwardAddress),
		TargetAddress:    stringValOrNull(api.TargetAddress),
		AllowedAddresses: stringSliceOrNil(api.AllowedAddresses),
		ForwardPort:      types.BoolValue(api.ForwardPort),
		AllowedPorts:     stringSliceOrNil(api.AllowedPorts),
		ZitiID:           stringValOrNull(api.ZitiID),
	}
	if api.TargetPort != nil {
		m.TargetPort = types.Int64Value(*api.TargetPort)
	} else {
		m.TargetPort = types.Int64Null()
	}
	return m
}

// addressesFromAPI maps API response addresses to the shared applicationAddressModel slice.
func addressesFromAPI(addresses []apiServiceAddress) []applicationAddressModel {
	result := make([]applicationAddressModel, len(addresses))
	for i, a := range addresses {
		result[i] = addressFromAPI(a)
	}
	return result
}

// applicationFromAPI maps an API response to the shared applicationModel.
func applicationFromAPI(api apiApplication) applicationModel {
	return applicationModel{
		ID:               types.StringValue(api.ID),
		ProviderID:       types.StringValue(api.ProviderID),
		ConnectorID:      types.StringValue(api.ConnectorID),
		ConnectorModelID: stringValOrNull(api.ConnectorModelID),
		Name:             types.StringValue(api.Name),
		Description:      stringValOrNull(api.Description),
		Type:             stringValOrNull(api.Type),
		Protocol:         stringValOrNull(api.Protocol),
		Enabled:          types.BoolValue(api.Enabled),
		ZitiID:           stringValOrNull(api.ZitiID),
		Addresses:        addressesFromAPI(api.Addresses),
		CreatedBy:        types.StringValue(api.CreatedBy),
		CreatedAt:        types.StringValue(api.CreatedAt),
		UpdatedAt:        types.StringValue(api.UpdatedAt),
		DeletedAt:        stringValOrNull(api.DeletedAt),
		DeletedBy:        stringValOrNull(api.DeletedBy),
		ZitiName:         stringValOrNull(api.ZitiName),
	}
}

// applicationAddressResourceAttrs returns the schema attributes for an address entry
// in the resource schema.
func applicationAddressResourceAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"listen_address": schema.ListAttribute{
			ElementType:         types.StringType,
			Required:            true,
			MarkdownDescription: "Listen addresses (hosts) for the intercept config.",
		},
		"listen_port": schema.ListAttribute{
			ElementType:         types.StringType,
			Required:            true,
			MarkdownDescription: "Listen ports or port ranges for the intercept config.",
		},
		"forward_address": schema.BoolAttribute{
			Optional:            true,
			Computed:            true,
			MarkdownDescription: "Forward the listening address.",
		},
		"target_address": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "Target address for the host config. Required when forward_address is false.",
		},
		"allowed_addresses": schema.ListAttribute{
			ElementType:         types.StringType,
			Optional:            true,
			MarkdownDescription: "Allowed forward addresses (IPs, CIDR blocks, or hostnames). Required when forward_address is true.",
		},
		"forward_port": schema.BoolAttribute{
			Optional:            true,
			Computed:            true,
			MarkdownDescription: "Forward the listening port.",
		},
		"target_port": schema.Int64Attribute{
			Optional:            true,
			MarkdownDescription: "Target port for the host config. Required when forward_port is false.",
		},
		"allowed_ports": schema.ListAttribute{
			ElementType:         types.StringType,
			Optional:            true,
			MarkdownDescription: "Allowed forward ports or port ranges. Required when forward_port is true.",
		},
		"ziti_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Ziti service id of the service backing this address.",
		},
	}
}

func (r *applicationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application"
}

func (r *applicationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a NetFoundry Application (backed by a Ziti service).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique identifier of the application.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"provider_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Provider this application belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"connector_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Connector this application belongs to. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"connector_model_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "If set, this application is inherited from the referenced connector model and cannot be modified directly.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Display name of the application.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Description of the application.",
			},
			"type": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Type of the application (e.g. HTTP, HTTPS, SSH) or 'custom'. Required for all connector types except SDK_EMBEDDED, where it is accepted but silently dropped.",
			},
			"protocol": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Protocol (`TCP`, `UDP`, or `TCP_UDP`). Required for all connector types except SDK_EMBEDDED, where it is accepted but silently dropped.",
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "When false, the application is not materialized into a Ziti service. Defaults to true.",
			},
			"ziti_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Ziti service id of the primary service backing this application. Null while the application is disabled.",
			},
			"addresses": schema.ListNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Service address mappings. Required for all connector types except SDK_EMBEDDED, where any supplied addresses are accepted but silently dropped.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: applicationAddressResourceAttrs(),
				},
			},
			"created_by": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identity that created this application.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when this application was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
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
				MarkdownDescription: "Unique name used for the primary Ziti service, formatted as \"<providerId>|<connectorId>|<name>\". Derived from `name`, so it may change on update.",
			},
		},
	}
}

func (r *applicationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data, ok := req.ProviderData.(*customerConnectData)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type",
			fmt.Sprintf("Expected *customerConnectData, got %T", req.ProviderData))
		return
	}
	r.client = data
}

func (r *applicationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan applicationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := createApplicationPayload{
		Name: plan.Name.ValueString(),
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		payload.Description = plan.Description.ValueString()
	}
	if !plan.Type.IsNull() && !plan.Type.IsUnknown() {
		payload.Type = plan.Type.ValueString()
	}
	if !plan.Protocol.IsNull() && !plan.Protocol.IsUnknown() {
		payload.Protocol = plan.Protocol.ValueString()
	}
	if !plan.Enabled.IsNull() && !plan.Enabled.IsUnknown() {
		v := plan.Enabled.ValueBool()
		payload.Enabled = &v
	}
	if plan.Addresses != nil {
		payload.Addresses = addressesToAPI(plan.Addresses)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		resp.Diagnostics.AddError("Serialization error", fmt.Sprintf("Failed to marshal create payload: %s", err))
		return
	}

	url := fmt.Sprintf("%s/connectors/%s/applications", r.client.apiBaseURL, plan.ConnectorID.ValueString())

	respBody, _, err := doRequest(ctx, http.MethodPost, url, r.client.accessToken, body)
	if err != nil {
		resp.Diagnostics.AddError("Create application failed", err.Error())
		return
	}

	var app apiApplication
	if err := json.Unmarshal(respBody, &app); err != nil {
		resp.Diagnostics.AddError("Parse error", fmt.Sprintf("Failed to parse create response: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, applicationFromAPI(app))...)
}

func (r *applicationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state applicationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/connectors/%s/applications/%s", r.client.apiBaseURL, state.ConnectorID.ValueString(), state.ID.ValueString())

	respBody, _, err := doRequest(ctx, http.MethodGet, url, r.client.accessToken, nil)
	if err != nil {
		if errors.Is(err, errNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read application failed", err.Error())
		return
	}

	var app apiApplication
	if err := json.Unmarshal(respBody, &app); err != nil {
		resp.Diagnostics.AddError("Parse error", fmt.Sprintf("Failed to parse read response: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, applicationFromAPI(app))...)
}

func (r *applicationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan applicationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := updateApplicationPayload{}
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		payload.Name = plan.Name.ValueString()
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		payload.Description = plan.Description.ValueString()
	}
	if !plan.Type.IsNull() && !plan.Type.IsUnknown() {
		payload.Type = plan.Type.ValueString()
	}
	if !plan.Protocol.IsNull() && !plan.Protocol.IsUnknown() {
		payload.Protocol = plan.Protocol.ValueString()
	}
	if !plan.Enabled.IsNull() && !plan.Enabled.IsUnknown() {
		v := plan.Enabled.ValueBool()
		payload.Enabled = &v
	}
	if plan.Addresses != nil {
		payload.Addresses = addressesToAPI(plan.Addresses)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		resp.Diagnostics.AddError("Serialization error", fmt.Sprintf("Failed to marshal update payload: %s", err))
		return
	}

	url := fmt.Sprintf("%s/connectors/%s/applications/%s", r.client.apiBaseURL, plan.ConnectorID.ValueString(), plan.ID.ValueString())

	respBody, _, err := doRequest(ctx, http.MethodPatch, url, r.client.accessToken, body)
	if err != nil {
		resp.Diagnostics.AddError("Update application failed", err.Error())
		return
	}

	var app apiApplication
	if err := json.Unmarshal(respBody, &app); err != nil {
		resp.Diagnostics.AddError("Parse error", fmt.Sprintf("Failed to parse update response: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, applicationFromAPI(app))...)
}

func (r *applicationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state applicationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/connectors/%s/applications/%s", r.client.apiBaseURL, state.ConnectorID.ValueString(), state.ID.ValueString())

	_, _, err := doRequest(ctx, http.MethodDelete, url, r.client.accessToken, nil)
	if err != nil && !errors.Is(err, errNotFound) {
		resp.Diagnostics.AddError("Delete application failed", err.Error())
	}
}

// ImportState accepts an import identifier of the form "connector_id/application_id",
// since an Application can only be looked up in the context of its Connector.
func (r *applicationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format: connector_id/application_id. Got: %q", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("connector_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}
