package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &locationResource{}
var _ resource.ResourceWithImportState = &locationResource{}

func NewLocationResource() resource.Resource {
	return &locationResource{}
}

type locationResource struct {
	client *customerConnectData
}

// locationModel is shared by both the resource and the data source.
type locationModel struct {
	ID              types.String  `tfsdk:"id"`
	CustomerID      types.String  `tfsdk:"customer_id"`
	ProviderID      types.String  `tfsdk:"provider_id"`
	Name            types.String  `tfsdk:"name"`
	Description     types.String  `tfsdk:"description"`
	Address         types.String  `tfsdk:"address"`
	Longitude       types.Float64 `tfsdk:"longitude"`
	Latitude        types.Float64 `tfsdk:"latitude"`
	Virtual         types.Bool    `tfsdk:"virtual"`
	CloudProvider   types.String  `tfsdk:"cloud_provider"`
	CloudRegion     types.String  `tfsdk:"cloud_region"`
	Enabled         types.Bool    `tfsdk:"enabled"`
	OwnerIdentityID types.String  `tfsdk:"owner_identity_id"`
	CreatedBy       types.String  `tfsdk:"created_by"`
	CreatedAt       types.String  `tfsdk:"created_at"`
	UpdatedAt       types.String  `tfsdk:"updated_at"`
	DeletedAt       types.String  `tfsdk:"deleted_at"`
	DeletedBy       types.String  `tfsdk:"deleted_by"`
	Deleted         types.Bool    `tfsdk:"deleted"`
}

// apiLocation mirrors the Location/LocationView JSON returned by the API.
type apiLocation struct {
	ID              string   `json:"id"`
	ProviderID      string   `json:"providerId"`
	CustomerID      string   `json:"customerId"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Virtual         bool     `json:"virtual"`
	Address         string   `json:"address"`
	Longitude       *float64 `json:"longitude"`
	Latitude        *float64 `json:"latitude"`
	CloudProvider   string   `json:"cloudProvider"`
	CloudRegion     string   `json:"cloudRegion"`
	Enabled         bool     `json:"enabled"`
	OwnerIdentityID string   `json:"ownerIdentityId"`
	CreatedBy       string   `json:"createdBy"`
	CreatedAt       string   `json:"createdAt"`
	UpdatedAt       string   `json:"updatedAt"`
	DeletedAt       string   `json:"deletedAt"`
	DeletedBy       string   `json:"deletedBy"`
	Deleted         bool     `json:"deleted"`
}

type createLocationPayload struct {
	CustomerID    string   `json:"customerId"`
	Name          string   `json:"name"`
	Description   string   `json:"description,omitempty"`
	Address       string   `json:"address,omitempty"`
	Longitude     *float64 `json:"longitude,omitempty"`
	Latitude      *float64 `json:"latitude,omitempty"`
	Virtual       *bool    `json:"virtual,omitempty"`
	CloudProvider string   `json:"cloudProvider,omitempty"`
	CloudRegion   string   `json:"cloudRegion,omitempty"`
}

type updateLocationPayload struct {
	Name          string   `json:"name,omitempty"`
	Description   string   `json:"description,omitempty"`
	Virtual       *bool    `json:"virtual,omitempty"`
	Address       string   `json:"address,omitempty"`
	Longitude     *float64 `json:"longitude,omitempty"`
	Latitude      *float64 `json:"latitude,omitempty"`
	CloudProvider string   `json:"cloudProvider,omitempty"`
	CloudRegion   string   `json:"cloudRegion,omitempty"`
	Enabled       *bool    `json:"enabled,omitempty"`
}

// locationFromAPI maps an API response to the shared locationModel.
func locationFromAPI(api apiLocation) locationModel {
	m := locationModel{
		ID:              types.StringValue(api.ID),
		ProviderID:      types.StringValue(api.ProviderID),
		CustomerID:      stringValOrNull(api.CustomerID), // omitted by API for provider-level locations
		Name:            types.StringValue(api.Name),
		Description:     stringValOrNull(api.Description),
		Address:         stringValOrNull(api.Address),
		Virtual:         types.BoolValue(api.Virtual),
		CloudProvider:   stringValOrNull(api.CloudProvider),
		CloudRegion:     stringValOrNull(api.CloudRegion),
		Enabled:         types.BoolValue(api.Enabled),
		OwnerIdentityID: types.StringValue(api.OwnerIdentityID),
		CreatedBy:       types.StringValue(api.CreatedBy),
		CreatedAt:       types.StringValue(api.CreatedAt),
		UpdatedAt:       types.StringValue(api.UpdatedAt),
		DeletedAt:       stringValOrNull(api.DeletedAt),
		DeletedBy:       stringValOrNull(api.DeletedBy),
		Deleted:         types.BoolValue(api.Deleted),
	}
	if api.Longitude != nil {
		m.Longitude = types.Float64Value(*api.Longitude)
	} else {
		m.Longitude = types.Float64Null()
	}
	if api.Latitude != nil {
		m.Latitude = types.Float64Value(*api.Latitude)
	} else {
		m.Latitude = types.Float64Null()
	}
	return m
}

func (r *locationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_location"
}

func (r *locationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a NetFoundry Location.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique identifier of the location.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"customer_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Customer this location belongs to. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"provider_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Provider this location belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the location.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Description of the location.",
			},
			"address": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Street address of the location.",
			},
			"longitude": schema.Float64Attribute{
				Optional:            true,
				MarkdownDescription: "Longitude in decimal degrees.",
			},
			"latitude": schema.Float64Attribute{
				Optional:            true,
				MarkdownDescription: "Latitude in decimal degrees.",
			},
			"virtual": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether this is a virtual location with no physical presence.",
			},
			"cloud_provider": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Cloud provider hosting this virtual location.",
			},
			"cloud_region": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Region of the cloud provider.",
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the location is enabled. Defaults to true on creation.",
			},
			"owner_identity_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identity that owns this location.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_by": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identity that created this location.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when this location was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when this location was last updated.",
			},
			"deleted_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when this location was deleted.",
			},
			"deleted_by": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identity that deleted this location.",
			},
			"deleted": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether this location has been deleted.",
			},
		},
	}
}

func (r *locationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	tflog.Debug(ctx, "Configuring location resource")
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

func (r *locationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Debug(ctx, "Creating location")

	var plan locationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := createLocationPayload{
		CustomerID: plan.CustomerID.ValueString(),
		Name:       plan.Name.ValueString(),
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		payload.Description = plan.Description.ValueString()
	}
	if !plan.Address.IsNull() && !plan.Address.IsUnknown() {
		payload.Address = plan.Address.ValueString()
	}
	if !plan.Longitude.IsNull() && !plan.Longitude.IsUnknown() {
		v := plan.Longitude.ValueFloat64()
		payload.Longitude = &v
	}
	if !plan.Latitude.IsNull() && !plan.Latitude.IsUnknown() {
		v := plan.Latitude.ValueFloat64()
		payload.Latitude = &v
	}
	if !plan.Virtual.IsNull() && !plan.Virtual.IsUnknown() {
		v := plan.Virtual.ValueBool()
		payload.Virtual = &v
	}
	if !plan.CloudProvider.IsNull() && !plan.CloudProvider.IsUnknown() {
		payload.CloudProvider = plan.CloudProvider.ValueString()
	}
	if !plan.CloudRegion.IsNull() && !plan.CloudRegion.IsUnknown() {
		payload.CloudRegion = plan.CloudRegion.ValueString()
	}

	body, err := json.Marshal(payload)
	if err != nil {
		resp.Diagnostics.AddError("Serialization error", fmt.Sprintf("Failed to marshal create payload: %s", err))
		return
	}

	url := fmt.Sprintf("%s/locations", r.client.apiBaseURL)

	respBody, _, err := doRequest(ctx, http.MethodPost, url, r.client.accessToken, body)
	if err != nil {
		resp.Diagnostics.AddError("Create location failed", err.Error())
		return
	}

	var loc apiLocation
	if err := json.Unmarshal(respBody, &loc); err != nil {
		resp.Diagnostics.AddError("Parse error", fmt.Sprintf("Failed to parse create response: %s", err))
		return
	}

	state := locationFromAPI(loc)
	// The API omits customerId for provider-level locations; preserve the plan value.
	if state.CustomerID.IsNull() {
		state.CustomerID = plan.CustomerID
	}

	// CreateLocation does not accept `enabled`; if the user wants disabled, PATCH immediately.
	if !plan.Enabled.IsNull() && !plan.Enabled.IsUnknown() && !plan.Enabled.ValueBool() {
		enabledFalse := false
		patchPayload := updateLocationPayload{Enabled: &enabledFalse}
		patchBody, _ := json.Marshal(patchPayload)
		patchURL := fmt.Sprintf("%s/locations/%s", r.client.apiBaseURL, loc.ID)
		patchResp, _, err := doRequest(ctx, http.MethodPatch, patchURL, r.client.accessToken, patchBody)
		if err != nil {
			resp.Diagnostics.AddError("Failed to disable location after creation", err.Error())
			return
		}
		var patched apiLocation
		if err := json.Unmarshal(patchResp, &patched); err != nil {
			resp.Diagnostics.AddError("Parse error", fmt.Sprintf("Failed to parse patch response: %s", err))
			return
		}
		state = locationFromAPI(patched)
		if state.CustomerID.IsNull() {
			state.CustomerID = plan.CustomerID
		}
	}

	tflog.Debug(ctx, "Created location", map[string]any{"id": state.ID.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *locationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state locationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading location", map[string]any{"id": state.ID.ValueString()})

	url := fmt.Sprintf("%s/locations/%s", r.client.apiBaseURL, state.ID.ValueString())

	respBody, _, err := doRequest(ctx, http.MethodGet, url, r.client.accessToken, nil)
	if err != nil {
		if errors.Is(err, errNotFound) {
			tflog.Debug(ctx, "Location not found, removing from state", map[string]any{"id": state.ID.ValueString()})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read location failed", err.Error())
		return
	}

	var loc apiLocation
	if err := json.Unmarshal(respBody, &loc); err != nil {
		resp.Diagnostics.AddError("Parse error", fmt.Sprintf("Failed to parse read response: %s", err))
		return
	}

	newState := locationFromAPI(loc)
	// Preserve customer_id from prior state when the API omits it.
	if newState.CustomerID.IsNull() {
		newState.CustomerID = state.CustomerID
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

func (r *locationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan locationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating location", map[string]any{"id": plan.ID.ValueString()})

	payload := updateLocationPayload{}
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		payload.Name = plan.Name.ValueString()
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		payload.Description = plan.Description.ValueString()
	}
	if !plan.Address.IsNull() && !plan.Address.IsUnknown() {
		payload.Address = plan.Address.ValueString()
	}
	if !plan.Longitude.IsNull() && !plan.Longitude.IsUnknown() {
		v := plan.Longitude.ValueFloat64()
		payload.Longitude = &v
	}
	if !plan.Latitude.IsNull() && !plan.Latitude.IsUnknown() {
		v := plan.Latitude.ValueFloat64()
		payload.Latitude = &v
	}
	if !plan.Virtual.IsNull() && !plan.Virtual.IsUnknown() {
		v := plan.Virtual.ValueBool()
		payload.Virtual = &v
	}
	if !plan.CloudProvider.IsNull() && !plan.CloudProvider.IsUnknown() {
		payload.CloudProvider = plan.CloudProvider.ValueString()
	}
	if !plan.CloudRegion.IsNull() && !plan.CloudRegion.IsUnknown() {
		payload.CloudRegion = plan.CloudRegion.ValueString()
	}
	if !plan.Enabled.IsNull() && !plan.Enabled.IsUnknown() {
		v := plan.Enabled.ValueBool()
		payload.Enabled = &v
	}

	body, err := json.Marshal(payload)
	if err != nil {
		resp.Diagnostics.AddError("Serialization error", fmt.Sprintf("Failed to marshal update payload: %s", err))
		return
	}

	url := fmt.Sprintf("%s/locations/%s", r.client.apiBaseURL, plan.ID.ValueString())

	respBody, _, err := doRequest(ctx, http.MethodPatch, url, r.client.accessToken, body)
	if err != nil {
		resp.Diagnostics.AddError("Update location failed", err.Error())
		return
	}

	var loc apiLocation
	if err := json.Unmarshal(respBody, &loc); err != nil {
		resp.Diagnostics.AddError("Parse error", fmt.Sprintf("Failed to parse update response: %s", err))
		return
	}

	newState := locationFromAPI(loc)
	if newState.CustomerID.IsNull() {
		newState.CustomerID = plan.CustomerID
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

func (r *locationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state locationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting location", map[string]any{"id": state.ID.ValueString()})

	url := fmt.Sprintf("%s/locations/%s", r.client.apiBaseURL, state.ID.ValueString())

	_, _, err := doRequest(ctx, http.MethodDelete, url, r.client.accessToken, nil)
	if err != nil && !errors.Is(err, errNotFound) {
		resp.Diagnostics.AddError("Delete location failed", err.Error())
	}
}

func (r *locationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Debug(ctx, "Importing location", map[string]any{"id": req.ID})
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
