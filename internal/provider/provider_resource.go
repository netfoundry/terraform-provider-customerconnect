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

var _ resource.Resource = &ccProviderResource{}
var _ resource.ResourceWithImportState = &ccProviderResource{}

func NewCCProviderResource() resource.Resource {
	return &ccProviderResource{}
}

type ccProviderResource struct {
	client *customerConnectData
}

// providerEntityModel is shared by both the resource and the data source.
type providerEntityModel struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	NetworkID          types.String `tfsdk:"network_id"`
	OrganizationID     types.String `tfsdk:"organization_id"`
	NetworkGroupID     types.String `tfsdk:"network_group_id"`
	InternalCustomerID types.String `tfsdk:"internal_customer_id"`
	OwnerIdentityID    types.String `tfsdk:"owner_identity_id"`
	CreatedBy          types.String `tfsdk:"created_by"`
	CreatedAt          types.String `tfsdk:"created_at"`
	UpdatedAt          types.String `tfsdk:"updated_at"`
	DeletedAt          types.String `tfsdk:"deleted_at"`
	DeletedBy          types.String `tfsdk:"deleted_by"`
	Deleted            types.Bool   `tfsdk:"deleted"`
	Counts             types.Object `tfsdk:"counts"`
}

// apiProviderCounts mirrors the counts JSON returned inline on a Provider.
type apiProviderCounts struct {
	Customers  int64 `json:"customers"`
	Locations  int64 `json:"locations"`
	Connectors int64 `json:"connectors"`
}

// apiProviderEntity mirrors the Provider/ProviderView JSON returned by the API.
type apiProviderEntity struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	NetworkID          string            `json:"networkId"`
	OrganizationID     string            `json:"organizationId"`
	NetworkGroupID     string            `json:"networkGroupId"`
	InternalCustomerID string            `json:"internalCustomerId"`
	OwnerIdentityID    string            `json:"ownerIdentityId"`
	CreatedBy          string            `json:"createdBy"`
	CreatedAt          apiTimestamp      `json:"createdAt"`
	UpdatedAt          apiTimestamp      `json:"updatedAt"`
	DeletedAt          apiTimestamp      `json:"deletedAt"`
	DeletedBy          string            `json:"deletedBy"`
	Deleted            bool              `json:"deleted"`
	Counts             apiProviderCounts `json:"counts"`
}

type createProviderEntityPayload struct {
	Name           string `json:"name"`
	NetworkID      string `json:"networkId"`
	OrganizationID string `json:"organizationId"`
}

type updateProviderEntityPayload struct {
	Name string `json:"name,omitempty"`
}

// providerCountsAttrTypes describes the object type of the "counts" attribute.
func providerCountsAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"customers":  types.Int64Type,
		"locations":  types.Int64Type,
		"connectors": types.Int64Type,
	}
}

// providerCountsFromAPI maps an API response to the "counts" object value.
func providerCountsFromAPI(api apiProviderCounts) types.Object {
	return types.ObjectValueMust(providerCountsAttrTypes(), map[string]attr.Value{
		"customers":  types.Int64Value(api.Customers),
		"locations":  types.Int64Value(api.Locations),
		"connectors": types.Int64Value(api.Connectors),
	})
}

// providerEntityFromAPI maps an API response to the shared providerEntityModel.
func providerEntityFromAPI(api apiProviderEntity) providerEntityModel {
	return providerEntityModel{
		ID:                 types.StringValue(api.ID),
		Name:               types.StringValue(api.Name),
		NetworkID:          types.StringValue(api.NetworkID),
		OrganizationID:     stringValOrNull(api.OrganizationID),
		NetworkGroupID:     types.StringValue(api.NetworkGroupID),
		InternalCustomerID: stringValOrNull(api.InternalCustomerID),
		OwnerIdentityID:    types.StringValue(api.OwnerIdentityID),
		CreatedBy:          types.StringValue(api.CreatedBy),
		CreatedAt:          types.StringValue(string(api.CreatedAt)),
		UpdatedAt:          types.StringValue(string(api.UpdatedAt)),
		DeletedAt:          stringValOrNull(string(api.DeletedAt)),
		DeletedBy:          stringValOrNull(api.DeletedBy),
		Deleted:            types.BoolValue(api.Deleted),
		Counts:             providerCountsFromAPI(api.Counts),
	}
}

// providerCountsResourceAttrs returns the schema attributes for the counts
// nested object in the resource schema.
func providerCountsResourceAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"customers": schema.Int64Attribute{
			Computed:            true,
			MarkdownDescription: "Number of customers under this provider.",
		},
		"locations": schema.Int64Attribute{
			Computed:            true,
			MarkdownDescription: "Number of locations under this provider.",
		},
		"connectors": schema.Int64Attribute{
			Computed:            true,
			MarkdownDescription: "Number of connectors under this provider.",
		},
	}
}

func (r *ccProviderResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_provider"
}

func (r *ccProviderResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a NetFoundry Customer Connect Provider — the top-level entity that owns connectors, connector models, locations and customers within a network.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique identifier of the provider.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the provider.",
			},
			"network_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Network identifier associated with this provider. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"organization_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Organisation identifier associated with this provider. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"network_group_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Network Group identifier associated with this provider.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"internal_customer_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Internal customer that holds locations attached directly to this provider.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"owner_identity_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identity that owns this provider.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_by": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identity that created this provider.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when this provider was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when this provider was last updated.",
			},
			"deleted_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when this provider was deleted.",
			},
			"deleted_by": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identity that deleted this provider.",
			},
			"deleted": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether this provider has been deleted.",
			},
			"counts": schema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Sub-resource counts for this provider.",
				Attributes:          providerCountsResourceAttrs(),
			},
		},
	}
}

func (r *ccProviderResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	tflog.Debug(ctx, "Configuring provider resource")
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

func (r *ccProviderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Debug(ctx, "Creating provider")

	var plan providerEntityModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := createProviderEntityPayload{
		Name:           plan.Name.ValueString(),
		NetworkID:      plan.NetworkID.ValueString(),
		OrganizationID: plan.OrganizationID.ValueString(),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		resp.Diagnostics.AddError("Serialization error", fmt.Sprintf("Failed to marshal create payload: %s", err))
		return
	}

	url := fmt.Sprintf("%s/providers", r.client.apiBaseURL)

	respBody, _, err := doRequest(ctx, http.MethodPost, url, r.client.accessToken, body)
	if err != nil {
		resp.Diagnostics.AddError("Create provider failed", err.Error())
		return
	}

	var p apiProviderEntity
	if err := json.Unmarshal(respBody, &p); err != nil {
		resp.Diagnostics.AddError("Parse error", fmt.Sprintf("Failed to parse create response: %s", err))
		return
	}

	state := providerEntityFromAPI(p)
	// The API may omit organizationId on the created provider; preserve the plan value.
	if state.OrganizationID.IsNull() {
		state.OrganizationID = plan.OrganizationID
	}

	tflog.Debug(ctx, "Created provider", map[string]any{"id": state.ID.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ccProviderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state providerEntityModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading provider", map[string]any{"id": state.ID.ValueString()})

	url := fmt.Sprintf("%s/providers/%s", r.client.apiBaseURL, state.ID.ValueString())

	respBody, _, err := doRequest(ctx, http.MethodGet, url, r.client.accessToken, nil)
	if err != nil {
		if errors.Is(err, errNotFound) {
			tflog.Debug(ctx, "Provider not found, removing from state", map[string]any{"id": state.ID.ValueString()})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read provider failed", err.Error())
		return
	}

	var p apiProviderEntity
	if err := json.Unmarshal(respBody, &p); err != nil {
		resp.Diagnostics.AddError("Parse error", fmt.Sprintf("Failed to parse read response: %s", err))
		return
	}

	newState := providerEntityFromAPI(p)
	// Preserve organization_id from prior state when the API omits it.
	if newState.OrganizationID.IsNull() {
		newState.OrganizationID = state.OrganizationID
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

func (r *ccProviderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan providerEntityModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating provider", map[string]any{"id": plan.ID.ValueString()})

	payload := updateProviderEntityPayload{
		Name: plan.Name.ValueString(),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		resp.Diagnostics.AddError("Serialization error", fmt.Sprintf("Failed to marshal update payload: %s", err))
		return
	}

	url := fmt.Sprintf("%s/providers/%s", r.client.apiBaseURL, plan.ID.ValueString())

	respBody, _, err := doRequest(ctx, http.MethodPatch, url, r.client.accessToken, body)
	if err != nil {
		resp.Diagnostics.AddError("Update provider failed", err.Error())
		return
	}

	var p apiProviderEntity
	if err := json.Unmarshal(respBody, &p); err != nil {
		resp.Diagnostics.AddError("Parse error", fmt.Sprintf("Failed to parse update response: %s", err))
		return
	}

	newState := providerEntityFromAPI(p)
	if newState.OrganizationID.IsNull() {
		newState.OrganizationID = plan.OrganizationID
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

func (r *ccProviderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state providerEntityModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting provider", map[string]any{"id": state.ID.ValueString()})

	url := fmt.Sprintf("%s/providers/%s", r.client.apiBaseURL, state.ID.ValueString())

	_, _, err := doRequest(ctx, http.MethodDelete, url, r.client.accessToken, nil)
	if err != nil && !errors.Is(err, errNotFound) {
		resp.Diagnostics.AddError("Delete provider failed", err.Error())
	}
}

func (r *ccProviderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Debug(ctx, "Importing provider", map[string]any{"id": req.ID})
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
