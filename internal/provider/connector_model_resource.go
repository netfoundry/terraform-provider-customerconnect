package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &connectorModelResource{}
var _ resource.ResourceWithImportState = &connectorModelResource{}

func NewConnectorModelResource() resource.Resource {
	return &connectorModelResource{}
}

type connectorModelResource struct {
	client *customerConnectData
}

// connectorModelAddressModel is shared by both the resource and the data source.
type connectorModelAddressModel struct {
	Key               types.String `tfsdk:"key"`
	ListenAddress     []string     `tfsdk:"listen_address"`
	ListenPort        []string     `tfsdk:"listen_port"`
	ForwardAddress    types.Bool   `tfsdk:"forward_address"`
	TargetAddress     types.String `tfsdk:"target_address"`
	AllowedAddresses  []string     `tfsdk:"allowed_addresses"`
	ForwardPort       types.Bool   `tfsdk:"forward_port"`
	TargetPort        types.String `tfsdk:"target_port"`
	AllowedPorts      []string     `tfsdk:"allowed_ports"`
	OverridableFields types.List   `tfsdk:"overridable_fields"`
	RequiredFields    types.List   `tfsdk:"required_fields"`
}

// connectorModelApplicationModel is shared by both the resource and the data source.
type connectorModelApplicationModel struct {
	Key               types.String                 `tfsdk:"key"`
	Name              types.String                 `tfsdk:"name"`
	Type              types.String                 `tfsdk:"type"`
	Protocol          types.String                 `tfsdk:"protocol"`
	Addresses         []connectorModelAddressModel `tfsdk:"addresses"`
	OverridableFields types.List                   `tfsdk:"overridable_fields"`
	RequiredFields    types.List                   `tfsdk:"required_fields"`
}

// connectorModelModel is shared by both the resource and the data source.
type connectorModelModel struct {
	ID              types.String                     `tfsdk:"id"`
	ProviderID      types.String                     `tfsdk:"provider_id"`
	Name            types.String                     `tfsdk:"name"`
	Description     types.String                     `tfsdk:"description"`
	Type            types.String                     `tfsdk:"type"`
	Applications    []connectorModelApplicationModel `tfsdk:"applications"`
	OwnerIdentityID types.String                     `tfsdk:"owner_identity_id"`
	CreatedBy       types.String                     `tfsdk:"created_by"`
	CreatedAt       types.String                     `tfsdk:"created_at"`
	UpdatedAt       types.String                     `tfsdk:"updated_at"`
	DeletedAt       types.String                     `tfsdk:"deleted_at"`
	DeletedBy       types.String                     `tfsdk:"deleted_by"`
	Deleted         types.Bool                       `tfsdk:"deleted"`
	Counts          types.Object                     `tfsdk:"counts"`
}

// apiModelApplicationAddress mirrors the ModelApplicationAddress JSON returned by the API.
type apiModelApplicationAddress struct {
	Key               string   `json:"key"`
	ListenAddress     []string `json:"listenAddress"`
	ListenPort        []string `json:"listenPort"`
	ForwardAddress    bool     `json:"forwardAddress"`
	TargetAddress     string   `json:"targetAddress"`
	AllowedAddresses  []string `json:"allowedAddresses"`
	ForwardPort       bool     `json:"forwardPort"`
	TargetPort        string   `json:"targetPort"`
	AllowedPorts      []string `json:"allowedPorts"`
	OverridableFields []string `json:"overridableFields"`
	RequiredFields    []string `json:"requiredFields"`
}

// apiModelApplication mirrors the ModelApplication JSON returned by the API.
type apiModelApplication struct {
	Key               string                       `json:"key"`
	Name              string                       `json:"name"`
	Type              string                       `json:"type"`
	Protocol          string                       `json:"protocol"`
	Addresses         []apiModelApplicationAddress `json:"addresses"`
	OverridableFields []string                     `json:"overridableFields"`
	RequiredFields    []string                     `json:"requiredFields"`
}

// apiConnectorModelCounts mirrors the Counts JSON returned by the API.
type apiConnectorModelCounts struct {
	Applications              int64 `json:"applications"`
	SourceAccessPolicies      int64 `json:"sourceAccessPolicies"`
	DestinationAccessPolicies int64 `json:"destinationAccessPolicies"`
	SourceConnections         int64 `json:"sourceConnections"`
	DestinationConnections    int64 `json:"destinationConnections"`
}

// apiConnectorModel mirrors the ConnectorModelView JSON returned by the API.
type apiConnectorModel struct {
	ID              string                  `json:"id"`
	ProviderID      string                  `json:"providerId"`
	Name            string                  `json:"name"`
	Description     string                  `json:"description"`
	Type            string                  `json:"type"`
	Applications    []apiModelApplication   `json:"applications"`
	OwnerIdentityID string                  `json:"ownerIdentityId"`
	CreatedBy       string                  `json:"createdBy"`
	CreatedAt       string                  `json:"createdAt"`
	UpdatedAt       string                  `json:"updatedAt"`
	DeletedAt       string                  `json:"deletedAt"`
	DeletedBy       string                  `json:"deletedBy"`
	Deleted         bool                    `json:"deleted"`
	Counts          apiConnectorModelCounts `json:"counts"`
}

// createModelAddressPayload is the wire format for creating/replacing a model application address.
type createModelAddressPayload struct {
	ListenAddress    []string `json:"listenAddress,omitempty"`
	ListenPort       []string `json:"listenPort,omitempty"`
	ForwardAddress   *bool    `json:"forwardAddress,omitempty"`
	TargetAddress    string   `json:"targetAddress,omitempty"`
	AllowedAddresses []string `json:"allowedAddresses,omitempty"`
	ForwardPort      *bool    `json:"forwardPort,omitempty"`
	TargetPort       string   `json:"targetPort,omitempty"`
	AllowedPorts     []string `json:"allowedPorts,omitempty"`
}

// createModelApplicationPayload is the wire format for creating/replacing a model application.
type createModelApplicationPayload struct {
	Name      string                      `json:"name"`
	Type      string                      `json:"type,omitempty"`
	Protocol  string                      `json:"protocol,omitempty"`
	Addresses []createModelAddressPayload `json:"addresses,omitempty"`
}

type createConnectorModelPayload struct {
	ProviderID   string                          `json:"providerId"`
	Name         string                          `json:"name"`
	Description  string                          `json:"description,omitempty"`
	Type         string                          `json:"type"`
	Applications []createModelApplicationPayload `json:"applications,omitempty"`
}

type updateConnectorModelPayload struct {
	Name         string                          `json:"name,omitempty"`
	Description  string                          `json:"description,omitempty"`
	Applications []createModelApplicationPayload `json:"applications,omitempty"`
}

// modelAddressToAPI converts an address model to the API wire format.
func modelAddressToAPI(m connectorModelAddressModel) createModelAddressPayload {
	p := createModelAddressPayload{
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
		p.TargetPort = m.TargetPort.ValueString()
	}
	if len(m.AllowedPorts) > 0 {
		p.AllowedPorts = m.AllowedPorts
	}
	return p
}

// modelApplicationToAPI converts an application model to the API wire format.
func modelApplicationToAPI(m connectorModelApplicationModel) createModelApplicationPayload {
	p := createModelApplicationPayload{
		Name: m.Name.ValueString(),
	}
	if !m.Type.IsNull() && !m.Type.IsUnknown() {
		p.Type = m.Type.ValueString()
	}
	if !m.Protocol.IsNull() && !m.Protocol.IsUnknown() {
		p.Protocol = m.Protocol.ValueString()
	}
	if m.Addresses != nil {
		addresses := make([]createModelAddressPayload, len(m.Addresses))
		for i, a := range m.Addresses {
			addresses[i] = modelAddressToAPI(a)
		}
		p.Addresses = addresses
	}
	return p
}

// modelApplicationsToAPI converts a slice of application models to the API wire format.
func modelApplicationsToAPI(applications []connectorModelApplicationModel) []createModelApplicationPayload {
	result := make([]createModelApplicationPayload, len(applications))
	for i, a := range applications {
		result[i] = modelApplicationToAPI(a)
	}
	return result
}

// stringListValue converts a Go string slice to a types.List, matching the
// null/known semantics expected for a Computed ListAttribute of strings.
func stringListValue(vals []string) types.List {
	if vals == nil {
		return types.ListNull(types.StringType)
	}
	elems := make([]attr.Value, len(vals))
	for i, v := range vals {
		elems[i] = types.StringValue(v)
	}
	return types.ListValueMust(types.StringType, elems)
}

// modelAddressFromAPI maps an API response to the shared connectorModelAddressModel.
func modelAddressFromAPI(api apiModelApplicationAddress) connectorModelAddressModel {
	return connectorModelAddressModel{
		Key:               stringValOrNull(api.Key),
		ListenAddress:     stringSliceOrNil(api.ListenAddress),
		ListenPort:        stringSliceOrNil(api.ListenPort),
		ForwardAddress:    types.BoolValue(api.ForwardAddress),
		TargetAddress:     stringValOrNull(api.TargetAddress),
		AllowedAddresses:  stringSliceOrNil(api.AllowedAddresses),
		ForwardPort:       types.BoolValue(api.ForwardPort),
		TargetPort:        stringValOrNull(api.TargetPort),
		AllowedPorts:      stringSliceOrNil(api.AllowedPorts),
		OverridableFields: stringListValue(api.OverridableFields),
		RequiredFields:    stringListValue(api.RequiredFields),
	}
}

// modelApplicationFromAPI maps an API response to the shared connectorModelApplicationModel.
func modelApplicationFromAPI(api apiModelApplication) connectorModelApplicationModel {
	var addresses []connectorModelAddressModel
	if len(api.Addresses) > 0 {
		addresses = make([]connectorModelAddressModel, len(api.Addresses))
		for i, a := range api.Addresses {
			addresses[i] = modelAddressFromAPI(a)
		}
	}
	return connectorModelApplicationModel{
		Key:               stringValOrNull(api.Key),
		Name:              types.StringValue(api.Name),
		Type:              stringValOrNull(api.Type),
		Protocol:          stringValOrNull(api.Protocol),
		Addresses:         addresses,
		OverridableFields: stringListValue(api.OverridableFields),
		RequiredFields:    stringListValue(api.RequiredFields),
	}
}

// modelApplicationsFromAPI maps API response applications to the shared connectorModelApplicationModel slice.
func modelApplicationsFromAPI(applications []apiModelApplication) []connectorModelApplicationModel {
	if len(applications) == 0 {
		return nil
	}
	result := make([]connectorModelApplicationModel, len(applications))
	for i, a := range applications {
		result[i] = modelApplicationFromAPI(a)
	}
	return result
}

// connectorModelCountsAttrTypes describes the object type of the "counts" attribute.
func connectorModelCountsAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"applications":                types.Int64Type,
		"source_access_policies":      types.Int64Type,
		"destination_access_policies": types.Int64Type,
		"source_connections":          types.Int64Type,
		"destination_connections":     types.Int64Type,
	}
}

// countsFromAPI maps an API response to the "counts" object value.
func countsFromAPI(api apiConnectorModelCounts) types.Object {
	return types.ObjectValueMust(connectorModelCountsAttrTypes(), map[string]attr.Value{
		"applications":                types.Int64Value(api.Applications),
		"source_access_policies":      types.Int64Value(api.SourceAccessPolicies),
		"destination_access_policies": types.Int64Value(api.DestinationAccessPolicies),
		"source_connections":          types.Int64Value(api.SourceConnections),
		"destination_connections":     types.Int64Value(api.DestinationConnections),
	})
}

// connectorModelFromAPI maps an API response to the shared connectorModelModel.
func connectorModelFromAPI(api apiConnectorModel) connectorModelModel {
	return connectorModelModel{
		ID:              types.StringValue(api.ID),
		ProviderID:      types.StringValue(api.ProviderID),
		Name:            types.StringValue(api.Name),
		Description:     stringValOrNull(api.Description),
		Type:            types.StringValue(api.Type),
		Applications:    modelApplicationsFromAPI(api.Applications),
		OwnerIdentityID: types.StringValue(api.OwnerIdentityID),
		CreatedBy:       types.StringValue(api.CreatedBy),
		CreatedAt:       types.StringValue(api.CreatedAt),
		UpdatedAt:       types.StringValue(api.UpdatedAt),
		DeletedAt:       stringValOrNull(api.DeletedAt),
		DeletedBy:       stringValOrNull(api.DeletedBy),
		Deleted:         types.BoolValue(api.Deleted),
		Counts:          countsFromAPI(api.Counts),
	}
}

// connectorModelAddressResourceAttrs returns the schema attributes for an address entry
// in the resource schema.
func connectorModelAddressResourceAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"key": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Server-assigned identifier for this address. Connector overrides bind to this key. Reassigned whenever the owning application's addresses are replaced.",
		},
		"listen_address": schema.ListAttribute{
			ElementType:         types.StringType,
			Optional:            true,
			MarkdownDescription: "Listen addresses (hosts) for the intercept config. Nullable — connector must supply via override if omitted.",
		},
		"listen_port": schema.ListAttribute{
			ElementType:         types.StringType,
			Optional:            true,
			MarkdownDescription: "Listen ports or port ranges for the intercept config. Nullable — connector must supply via override if omitted.",
		},
		"forward_address": schema.BoolAttribute{
			Optional:            true,
			Computed:            true,
			MarkdownDescription: "Whether the host config forwards the intercepted address. Defaults to false.",
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
			MarkdownDescription: "Whether the host config forwards the intercepted port. Defaults to false.",
		},
		"target_port": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "Target port for the host config (required when forward_port is false). A {{provider|customer|location}} template variable or a concrete port.",
		},
		"allowed_ports": schema.ListAttribute{
			ElementType:         types.StringType,
			Optional:            true,
			MarkdownDescription: "Allowed forward ports or port ranges. Required when forward_port is true.",
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

// connectorModelApplicationResourceAttrs returns the schema attributes for an application
// entry in the resource schema.
func connectorModelApplicationResourceAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"key": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Server-assigned identifier for this application. Connector overrides bind to this key. Reassigned whenever the connector model's applications are replaced.",
		},
		"name": schema.StringAttribute{
			Required:            true,
			MarkdownDescription: "Default name of the application. A connector override may replace it; the stable identity is `key`, not `name`.",
		},
		"type": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "Type of the application. Required unless the owning model is SDK_EMBEDDED. Must be a seeded ApplicationType name (e.g. HTTP, HTTPS, SSH) or 'custom'.",
		},
		"protocol": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "Protocol (`TCP`, `UDP`, or `TCP_UDP`). Required unless the owning model is SDK_EMBEDDED.",
		},
		"addresses": schema.ListNestedAttribute{
			Optional:            true,
			MarkdownDescription: "Service address mappings. Nullable — if omitted, the connector must supply addresses via overrides.",
			NestedObject: schema.NestedAttributeObject{
				Attributes: connectorModelAddressResourceAttrs(),
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

// connectorModelCountsResourceAttrs returns the schema attributes for the counts
// nested object in the resource schema.
func connectorModelCountsResourceAttrs() map[string]schema.Attribute {
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

func (r *connectorModelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_connector_model"
}

func (r *connectorModelResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a NetFoundry Connector Model — a reusable template defining the applications a connector hosts, from which connectors of a provider inherit their configuration.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique identifier of the connector model.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"provider_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Provider this connector model belongs to. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the connector model.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Description of the connector model.",
			},
			"type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Type of the connector model: `DEVICE`, `GATEWAY`, or `SDK_EMBEDDED`. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"applications": schema.ListNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Applications defined in this connector model. Providing this attribute always fully replaces the existing list.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: connectorModelApplicationResourceAttrs(),
				},
			},
			"owner_identity_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identity that owns this connector model.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_by": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identity that created this connector model.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when this connector model was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
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
				Attributes:          connectorModelCountsResourceAttrs(),
			},
		},
	}
}

func (r *connectorModelResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	tflog.Debug(ctx, "Configuring connector model resource")
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

func (r *connectorModelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Debug(ctx, "Creating connector model")

	var plan connectorModelModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := createConnectorModelPayload{
		ProviderID: plan.ProviderID.ValueString(),
		Name:       plan.Name.ValueString(),
		Type:       plan.Type.ValueString(),
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		payload.Description = plan.Description.ValueString()
	}
	if plan.Applications != nil {
		payload.Applications = modelApplicationsToAPI(plan.Applications)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		resp.Diagnostics.AddError("Serialization error", fmt.Sprintf("Failed to marshal create payload: %s", err))
		return
	}

	url := fmt.Sprintf("%s/connector-models", r.client.apiBaseURL)

	respBody, _, err := doRequest(ctx, http.MethodPost, url, r.client.accessToken, body)
	if err != nil {
		resp.Diagnostics.AddError("Create connector model failed", err.Error())
		return
	}

	var cm apiConnectorModel
	if err := json.Unmarshal(respBody, &cm); err != nil {
		resp.Diagnostics.AddError("Parse error", fmt.Sprintf("Failed to parse create response: %s", err))
		return
	}

	tflog.Debug(ctx, "Created connector model", map[string]any{"id": cm.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, connectorModelFromAPI(cm))...)
}

func (r *connectorModelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state connectorModelModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading connector model", map[string]any{"id": state.ID.ValueString()})

	url := fmt.Sprintf("%s/connector-models/%s", r.client.apiBaseURL, state.ID.ValueString())

	respBody, _, err := doRequest(ctx, http.MethodGet, url, r.client.accessToken, nil)
	if err != nil {
		if errors.Is(err, errNotFound) {
			tflog.Debug(ctx, "Connector model not found, removing from state", map[string]any{"id": state.ID.ValueString()})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read connector model failed", err.Error())
		return
	}

	var cm apiConnectorModel
	if err := json.Unmarshal(respBody, &cm); err != nil {
		resp.Diagnostics.AddError("Parse error", fmt.Sprintf("Failed to parse read response: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, connectorModelFromAPI(cm))...)
}

func (r *connectorModelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan connectorModelModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating connector model", map[string]any{"id": plan.ID.ValueString()})

	payload := updateConnectorModelPayload{
		Name: plan.Name.ValueString(),
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		payload.Description = plan.Description.ValueString()
	}
	if plan.Applications != nil {
		payload.Applications = modelApplicationsToAPI(plan.Applications)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		resp.Diagnostics.AddError("Serialization error", fmt.Sprintf("Failed to marshal update payload: %s", err))
		return
	}

	url := fmt.Sprintf("%s/connector-models/%s", r.client.apiBaseURL, plan.ID.ValueString())

	respBody, _, err := doRequest(ctx, http.MethodPatch, url, r.client.accessToken, body)
	if err != nil {
		resp.Diagnostics.AddError("Update connector model failed", err.Error())
		return
	}

	var cm apiConnectorModel
	if err := json.Unmarshal(respBody, &cm); err != nil {
		resp.Diagnostics.AddError("Parse error", fmt.Sprintf("Failed to parse update response: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, connectorModelFromAPI(cm))...)
}

func (r *connectorModelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state connectorModelModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting connector model", map[string]any{"id": state.ID.ValueString()})

	url := fmt.Sprintf("%s/connector-models/%s", r.client.apiBaseURL, state.ID.ValueString())

	_, _, err := doRequest(ctx, http.MethodDelete, url, r.client.accessToken, nil)
	if err != nil && !errors.Is(err, errNotFound) {
		resp.Diagnostics.AddError("Delete connector model failed", err.Error())
	}
}

func (r *connectorModelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Debug(ctx, "Importing connector model", map[string]any{"id": req.ID})
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
