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

var _ resource.Resource = &customerResource{}
var _ resource.ResourceWithImportState = &customerResource{}

func NewCustomerResource() resource.Resource {
	return &customerResource{}
}

type customerResource struct {
	client *customerConnectData
}

// customerModel is shared by both the resource and the data source.
type customerModel struct {
	ID              types.String `tfsdk:"id"`
	ProviderID      types.String `tfsdk:"provider_id"`
	Name            types.String `tfsdk:"name"`
	Description     types.String `tfsdk:"description"`
	Enabled         types.Bool   `tfsdk:"enabled"`
	OwnerIdentityID types.String `tfsdk:"owner_identity_id"`
	CreatedBy       types.String `tfsdk:"created_by"`
	CreatedAt       types.String `tfsdk:"created_at"`
	UpdatedAt       types.String `tfsdk:"updated_at"`
	DeletedAt       types.String `tfsdk:"deleted_at"`
	DeletedBy       types.String `tfsdk:"deleted_by"`
	Deleted         types.Bool   `tfsdk:"deleted"`
	Counts          types.Object `tfsdk:"counts"`
}

// apiCustomerCounts mirrors the Counts JSON returned by the API.
type apiCustomerCounts struct {
	Locations  int64 `json:"locations"`
	Connectors int64 `json:"connectors"`
}

// apiCustomer mirrors the Customer JSON returned by the API.
type apiCustomer struct {
	ID              string            `json:"id"`
	ProviderID      string            `json:"providerId"`
	Name            string            `json:"name"`
	Description     string            `json:"description"`
	Enabled         bool              `json:"enabled"`
	OwnerIdentityID string            `json:"ownerIdentityId"`
	CreatedBy       string            `json:"createdBy"`
	CreatedAt       string            `json:"createdAt"`
	UpdatedAt       string            `json:"updatedAt"`
	DeletedAt       string            `json:"deletedAt"`
	DeletedBy       string            `json:"deletedBy"`
	Deleted         bool              `json:"deleted"`
	Counts          apiCustomerCounts `json:"counts"`
}

type createCustomerPayload struct {
	ProviderID  string `json:"providerId"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type updateCustomerPayload struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Enabled     *bool  `json:"enabled,omitempty"`
}

// customerCountsAttrTypes describes the object type of the "counts" attribute.
func customerCountsAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"locations":  types.Int64Type,
		"connectors": types.Int64Type,
	}
}

// customerCountsFromAPI maps an API response to the "counts" object value.
func customerCountsFromAPI(api apiCustomerCounts) types.Object {
	return types.ObjectValueMust(customerCountsAttrTypes(), map[string]attr.Value{
		"locations":  types.Int64Value(api.Locations),
		"connectors": types.Int64Value(api.Connectors),
	})
}

// customerFromAPI maps an API response to the shared customerModel.
func customerFromAPI(api apiCustomer) customerModel {
	return customerModel{
		ID:              types.StringValue(api.ID),
		ProviderID:      types.StringValue(api.ProviderID),
		Name:            types.StringValue(api.Name),
		Description:     stringValOrNull(api.Description),
		Enabled:         types.BoolValue(api.Enabled),
		OwnerIdentityID: types.StringValue(api.OwnerIdentityID),
		CreatedBy:       types.StringValue(api.CreatedBy),
		CreatedAt:       types.StringValue(api.CreatedAt),
		UpdatedAt:       types.StringValue(api.UpdatedAt),
		DeletedAt:       stringValOrNull(api.DeletedAt),
		DeletedBy:       stringValOrNull(api.DeletedBy),
		Deleted:         types.BoolValue(api.Deleted),
		Counts:          customerCountsFromAPI(api.Counts),
	}
}

// customerCountsResourceAttrs returns the schema attributes for the counts
// nested object in the resource schema.
func customerCountsResourceAttrs() map[string]schema.Attribute {
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

func (r *customerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_customer"
}

func (r *customerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a NetFoundry Customer — a grouping of the locations and connectors that participate in access policies under a provider.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique identifier of the customer.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"provider_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Provider this customer belongs to. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the customer.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Description of the customer.",
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the customer is enabled. Defaults to true on creation. When false, every access policy touching this customer (via any of its locations or connectors) is suspended.",
			},
			"owner_identity_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identity that owns this customer.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_by": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identity that created this customer.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when this customer was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
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
				Attributes:          customerCountsResourceAttrs(),
			},
		},
	}
}

func (r *customerResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	tflog.Debug(ctx, "Configuring customer resource")
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

func (r *customerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Debug(ctx, "Creating customer")

	var plan customerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := createCustomerPayload{
		ProviderID: plan.ProviderID.ValueString(),
		Name:       plan.Name.ValueString(),
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		payload.Description = plan.Description.ValueString()
	}

	body, err := json.Marshal(payload)
	if err != nil {
		resp.Diagnostics.AddError("Serialization error", fmt.Sprintf("Failed to marshal create payload: %s", err))
		return
	}

	url := fmt.Sprintf("%s/customers", r.client.apiBaseURL)

	respBody, _, err := doRequest(ctx, http.MethodPost, url, r.client.accessToken, body)
	if err != nil {
		resp.Diagnostics.AddError("Create customer failed", err.Error())
		return
	}

	var c apiCustomer
	if err := json.Unmarshal(respBody, &c); err != nil {
		resp.Diagnostics.AddError("Parse error", fmt.Sprintf("Failed to parse create response: %s", err))
		return
	}

	state := customerFromAPI(c)

	// CreateCustomer does not accept `enabled`; if the user wants disabled, PATCH immediately.
	if !plan.Enabled.IsNull() && !plan.Enabled.IsUnknown() && !plan.Enabled.ValueBool() {
		enabledFalse := false
		patchPayload := updateCustomerPayload{Enabled: &enabledFalse}
		patchBody, _ := json.Marshal(patchPayload)
		patchURL := fmt.Sprintf("%s/customers/%s", r.client.apiBaseURL, c.ID)
		patchResp, _, err := doRequest(ctx, http.MethodPatch, patchURL, r.client.accessToken, patchBody)
		if err != nil {
			resp.Diagnostics.AddError("Failed to disable customer after creation", err.Error())
			return
		}
		var patched apiCustomer
		if err := json.Unmarshal(patchResp, &patched); err != nil {
			resp.Diagnostics.AddError("Parse error", fmt.Sprintf("Failed to parse patch response: %s", err))
			return
		}
		state = customerFromAPI(patched)
	}

	tflog.Debug(ctx, "Created customer", map[string]any{"id": state.ID.ValueString()})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *customerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state customerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading customer", map[string]any{"id": state.ID.ValueString()})

	url := fmt.Sprintf("%s/customers/%s", r.client.apiBaseURL, state.ID.ValueString())

	respBody, _, err := doRequest(ctx, http.MethodGet, url, r.client.accessToken, nil)
	if err != nil {
		if errors.Is(err, errNotFound) {
			tflog.Debug(ctx, "Customer not found, removing from state", map[string]any{"id": state.ID.ValueString()})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read customer failed", err.Error())
		return
	}

	var c apiCustomer
	if err := json.Unmarshal(respBody, &c); err != nil {
		resp.Diagnostics.AddError("Parse error", fmt.Sprintf("Failed to parse read response: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, customerFromAPI(c))...)
}

func (r *customerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan customerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating customer", map[string]any{"id": plan.ID.ValueString()})

	payload := updateCustomerPayload{
		Name: plan.Name.ValueString(),
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		payload.Description = plan.Description.ValueString()
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

	url := fmt.Sprintf("%s/customers/%s", r.client.apiBaseURL, plan.ID.ValueString())

	respBody, _, err := doRequest(ctx, http.MethodPatch, url, r.client.accessToken, body)
	if err != nil {
		resp.Diagnostics.AddError("Update customer failed", err.Error())
		return
	}

	var c apiCustomer
	if err := json.Unmarshal(respBody, &c); err != nil {
		resp.Diagnostics.AddError("Parse error", fmt.Sprintf("Failed to parse update response: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, customerFromAPI(c))...)
}

func (r *customerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state customerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting customer", map[string]any{"id": state.ID.ValueString()})

	url := fmt.Sprintf("%s/customers/%s", r.client.apiBaseURL, state.ID.ValueString())

	_, _, err := doRequest(ctx, http.MethodDelete, url, r.client.accessToken, nil)
	if err != nil && !errors.Is(err, errNotFound) {
		resp.Diagnostics.AddError("Delete customer failed", err.Error())
	}
}

func (r *customerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Debug(ctx, "Importing customer", map[string]any{"id": req.ID})
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
