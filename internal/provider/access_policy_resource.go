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

var _ resource.Resource = &accessPolicyResource{}
var _ resource.ResourceWithImportState = &accessPolicyResource{}

func NewAccessPolicyResource() resource.Resource {
	return &accessPolicyResource{}
}

type accessPolicyResource struct {
	client *customerConnectData
}

// accessPolicyEndpointModel represents one source or destination endpoint.
// Exactly one of the three IDs must be set.
type accessPolicyEndpointModel struct {
	ConnectorID      types.String `tfsdk:"connector_id"`
	LocationID       types.String `tfsdk:"location_id"`
	ConnectorModelID types.String `tfsdk:"connector_model_id"`
}

// accessPolicyModel is shared by both the resource and the data source.
type accessPolicyModel struct {
	ID           types.String                `tfsdk:"id"`
	ProviderID   types.String                `tfsdk:"provider_id"`
	Name         types.String                `tfsdk:"name"`
	Description  types.String                `tfsdk:"description"`
	Sources      []accessPolicyEndpointModel `tfsdk:"sources"`
	Destinations []accessPolicyEndpointModel `tfsdk:"destinations"`
	Enabled      types.Bool                  `tfsdk:"enabled"`
	CreatedBy    types.String                `tfsdk:"created_by"`
	CreatedAt    types.String                `tfsdk:"created_at"`
	UpdatedAt    types.String                `tfsdk:"updated_at"`
	DeletedAt    types.String                `tfsdk:"deleted_at"`
	DeletedBy    types.String                `tfsdk:"deleted_by"`
	ZitiName     types.String                `tfsdk:"ziti_name"`
}

// apiAccessPolicyEndpoint mirrors Source/Destination in the API JSON.
type apiAccessPolicyEndpoint struct {
	ConnectorID      string `json:"connectorId,omitempty"`
	LocationID       string `json:"locationId,omitempty"`
	ConnectorModelID string `json:"connectorModelId,omitempty"`
}

// apiAccessPolicy mirrors the AccessPolicy JSON returned by the API.
type apiAccessPolicy struct {
	ID           string                    `json:"id"`
	ProviderID   string                    `json:"providerId"`
	Name         string                    `json:"name"`
	Description  string                    `json:"description"`
	Sources      []apiAccessPolicyEndpoint `json:"sources"`
	Destinations []apiAccessPolicyEndpoint `json:"destinations"`
	Enabled      bool                      `json:"enabled"`
	CreatedBy    string                    `json:"createdBy"`
	CreatedAt    string                    `json:"createdAt"`
	UpdatedAt    string                    `json:"updatedAt"`
	DeletedAt    string                    `json:"deletedAt"`
	DeletedBy    string                    `json:"deletedBy"`
	ZitiName     string                    `json:"zitiName"`
}

type createAccessPolicyPayload struct {
	ProviderID   string                    `json:"providerId"`
	Name         string                    `json:"name"`
	Description  string                    `json:"description,omitempty"`
	Sources      []apiAccessPolicyEndpoint `json:"sources"`
	Destinations []apiAccessPolicyEndpoint `json:"destinations"`
}

type updateAccessPolicyPayload struct {
	Name         string                    `json:"name,omitempty"`
	Description  string                    `json:"description,omitempty"`
	Sources      []apiAccessPolicyEndpoint `json:"sources,omitempty"`
	Destinations []apiAccessPolicyEndpoint `json:"destinations,omitempty"`
}

// endpointsToAPI converts a slice of endpoint models to the API wire format.
func endpointsToAPI(endpoints []accessPolicyEndpointModel) []apiAccessPolicyEndpoint {
	result := make([]apiAccessPolicyEndpoint, len(endpoints))
	for i, e := range endpoints {
		result[i] = apiAccessPolicyEndpoint{
			ConnectorID:      e.ConnectorID.ValueString(),
			LocationID:       e.LocationID.ValueString(),
			ConnectorModelID: e.ConnectorModelID.ValueString(),
		}
	}
	return result
}

// accessPolicyFromAPI maps an API response to the shared accessPolicyModel.
func accessPolicyFromAPI(api apiAccessPolicy) accessPolicyModel {
	sources := make([]accessPolicyEndpointModel, len(api.Sources))
	for i, s := range api.Sources {
		sources[i] = accessPolicyEndpointModel{
			ConnectorID:      stringValOrNull(s.ConnectorID),
			LocationID:       stringValOrNull(s.LocationID),
			ConnectorModelID: stringValOrNull(s.ConnectorModelID),
		}
	}

	destinations := make([]accessPolicyEndpointModel, len(api.Destinations))
	for i, d := range api.Destinations {
		destinations[i] = accessPolicyEndpointModel{
			ConnectorID:      stringValOrNull(d.ConnectorID),
			LocationID:       stringValOrNull(d.LocationID),
			ConnectorModelID: stringValOrNull(d.ConnectorModelID),
		}
	}

	return accessPolicyModel{
		ID:           types.StringValue(api.ID),
		ProviderID:   types.StringValue(api.ProviderID),
		Name:         types.StringValue(api.Name),
		Description:  stringValOrNull(api.Description),
		Sources:      sources,
		Destinations: destinations,
		Enabled:      types.BoolValue(api.Enabled),
		CreatedBy:    types.StringValue(api.CreatedBy),
		CreatedAt:    types.StringValue(api.CreatedAt),
		UpdatedAt:    types.StringValue(api.UpdatedAt),
		DeletedAt:    stringValOrNull(api.DeletedAt),
		DeletedBy:    stringValOrNull(api.DeletedBy),
		ZitiName:     stringValOrNull(api.ZitiName),
	}
}

// endpointNestedAttrs returns the schema attributes shared by the sources and
// destinations nested objects in both the resource and the data source.
func endpointNestedAttrs(computed bool) map[string]schema.Attribute {
	if computed {
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
	return map[string]schema.Attribute{
		"connector_id": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "Source/destination connector UUID.",
		},
		"location_id": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "Source/destination location UUID.",
		},
		"connector_model_id": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "Source/destination connector model UUID — matches any connector attached to the given model.",
		},
	}
}

func (r *accessPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_access_policy"
}

func (r *accessPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a NetFoundry AccessPolicy (backed by a Ziti dial service policy).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique identifier of the access policy.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"provider_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Provider this access policy belongs to. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Display name of the access policy.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Description of the access policy.",
			},
			"sources": schema.ListNestedAttribute{
				Required:            true,
				MarkdownDescription: "Source endpoints. Each entry must have exactly one of connector_id, location_id, or connector_model_id set.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: endpointNestedAttrs(false),
				},
			},
			"destinations": schema.ListNestedAttribute{
				Required:            true,
				MarkdownDescription: "Destination endpoints. Each entry must have exactly one of connector_id, location_id, or connector_model_id set.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: endpointNestedAttrs(false),
				},
			},
			"enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the access policy is currently projected onto the network. Controlled by the API based on linked resource states.",
			},
			"created_by": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identity that created this access policy.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when this access policy was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
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
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *accessPolicyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	tflog.Debug(ctx, "Configuring access policy resource")
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

func (r *accessPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Debug(ctx, "Creating access policy")

	var plan accessPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := createAccessPolicyPayload{
		ProviderID:   plan.ProviderID.ValueString(),
		Name:         plan.Name.ValueString(),
		Sources:      endpointsToAPI(plan.Sources),
		Destinations: endpointsToAPI(plan.Destinations),
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		payload.Description = plan.Description.ValueString()
	}

	body, err := json.Marshal(payload)
	if err != nil {
		resp.Diagnostics.AddError("Serialization error", fmt.Sprintf("Failed to marshal create payload: %s", err))
		return
	}

	url := fmt.Sprintf("%s/access-policies", r.client.apiBaseURL)

	respBody, _, err := doRequest(ctx, http.MethodPost, url, r.client.accessToken, body)
	if err != nil {
		resp.Diagnostics.AddError("Create access policy failed", err.Error())
		return
	}

	var ap apiAccessPolicy
	if err := json.Unmarshal(respBody, &ap); err != nil {
		resp.Diagnostics.AddError("Parse error", fmt.Sprintf("Failed to parse create response: %s", err))
		return
	}

	tflog.Debug(ctx, "Created access policy", map[string]any{"id": ap.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, accessPolicyFromAPI(ap))...)
}

func (r *accessPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state accessPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Reading access policy", map[string]any{"id": state.ID.ValueString()})

	url := fmt.Sprintf("%s/access-policies/%s", r.client.apiBaseURL, state.ID.ValueString())

	respBody, _, err := doRequest(ctx, http.MethodGet, url, r.client.accessToken, nil)
	if err != nil {
		if errors.Is(err, errNotFound) {
			tflog.Debug(ctx, "Access policy not found, removing from state", map[string]any{"id": state.ID.ValueString()})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read access policy failed", err.Error())
		return
	}

	var ap apiAccessPolicy
	if err := json.Unmarshal(respBody, &ap); err != nil {
		resp.Diagnostics.AddError("Parse error", fmt.Sprintf("Failed to parse read response: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, accessPolicyFromAPI(ap))...)
}

func (r *accessPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan accessPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating access policy", map[string]any{"id": plan.ID.ValueString()})

	payload := updateAccessPolicyPayload{
		Name:         plan.Name.ValueString(),
		Sources:      endpointsToAPI(plan.Sources),
		Destinations: endpointsToAPI(plan.Destinations),
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		payload.Description = plan.Description.ValueString()
	}

	body, err := json.Marshal(payload)
	if err != nil {
		resp.Diagnostics.AddError("Serialization error", fmt.Sprintf("Failed to marshal update payload: %s", err))
		return
	}

	url := fmt.Sprintf("%s/access-policies/%s", r.client.apiBaseURL, plan.ID.ValueString())

	respBody, _, err := doRequest(ctx, http.MethodPatch, url, r.client.accessToken, body)
	if err != nil {
		resp.Diagnostics.AddError("Update access policy failed", err.Error())
		return
	}

	var ap apiAccessPolicy
	if err := json.Unmarshal(respBody, &ap); err != nil {
		resp.Diagnostics.AddError("Parse error", fmt.Sprintf("Failed to parse update response: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, accessPolicyFromAPI(ap))...)
}

func (r *accessPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state accessPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting access policy", map[string]any{"id": state.ID.ValueString()})

	url := fmt.Sprintf("%s/access-policies/%s", r.client.apiBaseURL, state.ID.ValueString())

	_, _, err := doRequest(ctx, http.MethodDelete, url, r.client.accessToken, nil)
	if err != nil && !errors.Is(err, errNotFound) {
		resp.Diagnostics.AddError("Delete access policy failed", err.Error())
	}
}

func (r *accessPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Debug(ctx, "Importing access policy", map[string]any{"id": req.ID})
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
