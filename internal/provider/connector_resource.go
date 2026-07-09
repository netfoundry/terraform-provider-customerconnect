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
)

var _ resource.Resource = &connectorResource{}
var _ resource.ResourceWithImportState = &connectorResource{}

func NewConnectorResource() resource.Resource {
	return &connectorResource{}
}

type connectorResource struct {
	client *customerConnectData
}

// connectorModel is shared by both the resource and the data source.
type connectorModel struct {
	ID                  types.String `tfsdk:"id"`
	ProviderID          types.String `tfsdk:"provider_id"`
	CustomerID          types.String `tfsdk:"customer_id"`
	LocationID          types.String `tfsdk:"location_id"`
	Name                types.String `tfsdk:"name"`
	Description         types.String `tfsdk:"description"`
	Type                types.String `tfsdk:"type"`
	ConnectorModelID    types.String `tfsdk:"connector_model_id"`
	Enabled             types.Bool   `tfsdk:"enabled"`
	OwnerIdentityID     types.String `tfsdk:"owner_identity_id"`
	CreatedBy           types.String `tfsdk:"created_by"`
	CreatedAt           types.String `tfsdk:"created_at"`
	UpdatedAt           types.String `tfsdk:"updated_at"`
	DeletedAt           types.String `tfsdk:"deleted_at"`
	DeletedBy           types.String `tfsdk:"deleted_by"`
	Deleted             types.Bool   `tfsdk:"deleted"`
	ZitiID              types.String `tfsdk:"ziti_id"`
	ZitiName            types.String `tfsdk:"ziti_name"`
	Online              types.Bool   `tfsdk:"online"`
	Enrolled            types.Bool   `tfsdk:"enrolled"`
	EnrollmentJwt       types.String `tfsdk:"enrollment_jwt"`
	EnrollmentExpiresAt types.String `tfsdk:"enrollment_expires_at"`
}

// apiConnector mirrors the ConnectorView JSON returned by the API.
type apiConnector struct {
	ID                  string `json:"id"`
	ProviderID          string `json:"providerId"`
	CustomerID          string `json:"customerId"`
	LocationID          string `json:"locationId"`
	Name                string `json:"name"`
	Description         string `json:"description"`
	Type                string `json:"type"`
	ConnectorModelID    string `json:"connectorModelId"`
	Enabled             bool   `json:"enabled"`
	OwnerIdentityID     string `json:"ownerIdentityId"`
	CreatedBy           string `json:"createdBy"`
	CreatedAt           string `json:"createdAt"`
	UpdatedAt           string `json:"updatedAt"`
	DeletedAt           string `json:"deletedAt"`
	DeletedBy           string `json:"deletedBy"`
	Deleted             bool   `json:"deleted"`
	ZitiID              string `json:"zitiId"`
	ZitiName            string `json:"zitiName"`
	Online              bool   `json:"online"`
	Enrolled            bool   `json:"enrolled"`
	EnrollmentJwt       string `json:"enrollmentJwt"`
	EnrollmentExpiresAt string `json:"enrollmentExpiresAt"`
}

type createConnectorPayload struct {
	LocationID       string `json:"locationId"`
	Name             string `json:"name"`
	Description      string `json:"description,omitempty"`
	Type             string `json:"type,omitempty"`
	ConnectorModelID string `json:"connectorModelId,omitempty"`
}

type updateConnectorPayload struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Enabled     *bool  `json:"enabled,omitempty"`
}

// connectorFromAPI maps an API response to the shared connectorModel.
func connectorFromAPI(api apiConnector) connectorModel {
	return connectorModel{
		ID:                  types.StringValue(api.ID),
		ProviderID:          types.StringValue(api.ProviderID),
		CustomerID:          stringValOrNull(api.CustomerID),
		LocationID:          types.StringValue(api.LocationID),
		Name:                types.StringValue(api.Name),
		Description:         stringValOrNull(api.Description),
		Type:                stringValOrNull(api.Type),
		ConnectorModelID:    stringValOrNull(api.ConnectorModelID),
		Enabled:             types.BoolValue(api.Enabled),
		OwnerIdentityID:     types.StringValue(api.OwnerIdentityID),
		CreatedBy:           types.StringValue(api.CreatedBy),
		CreatedAt:           types.StringValue(api.CreatedAt),
		UpdatedAt:           types.StringValue(api.UpdatedAt),
		DeletedAt:           stringValOrNull(api.DeletedAt),
		DeletedBy:           stringValOrNull(api.DeletedBy),
		Deleted:             types.BoolValue(api.Deleted),
		ZitiID:              stringValOrNull(api.ZitiID),
		ZitiName:            stringValOrNull(api.ZitiName),
		Online:              types.BoolValue(api.Online),
		Enrolled:            types.BoolValue(api.Enrolled),
		EnrollmentJwt:       stringValOrNull(api.EnrollmentJwt),
		EnrollmentExpiresAt: stringValOrNull(api.EnrollmentExpiresAt),
	}
}

func (r *connectorResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_connector"
}

func (r *connectorResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a NetFoundry Connector.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique identifier of the connector.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"provider_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Provider this connector belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"customer_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Customer this connector belongs to. Omitted when the connector is hosted directly by the provider.",
			},
			"location_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Location this connector belongs to. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the connector.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Description of the connector.",
			},
			"type": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Type of the connector: `DEVICE`, `GATEWAY`, or `SDK_EMBEDDED`. Required unless `connector_model_id` is set, in which case the type is derived from the model.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"connector_model_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Connector model to inherit applications from.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the connector is enabled. Defaults to true on creation.",
			},
			"owner_identity_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identity that owns this connector.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_by": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identity that created this connector.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when this connector was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
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
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"ziti_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique name used for the Ziti identity.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
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

func (r *connectorResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *connectorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan connectorModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.Type.IsNull() && plan.ConnectorModelID.IsNull() {
		resp.Diagnostics.AddError("Missing required attribute",
			"Either 'type' or 'connector_model_id' must be specified when creating a connector.")
		return
	}

	payload := createConnectorPayload{
		LocationID: plan.LocationID.ValueString(),
		Name:       plan.Name.ValueString(),
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		payload.Description = plan.Description.ValueString()
	}
	if !plan.Type.IsNull() && !plan.Type.IsUnknown() {
		payload.Type = plan.Type.ValueString()
	}
	if !plan.ConnectorModelID.IsNull() && !plan.ConnectorModelID.IsUnknown() {
		payload.ConnectorModelID = plan.ConnectorModelID.ValueString()
	}

	body, err := json.Marshal(payload)
	if err != nil {
		resp.Diagnostics.AddError("Serialization error", fmt.Sprintf("Failed to marshal create payload: %s", err))
		return
	}

	url := fmt.Sprintf("%s/connectors", r.client.apiBaseURL)

	respBody, _, err := doRequest(ctx, http.MethodPost, url, r.client.accessToken, body)
	if err != nil {
		resp.Diagnostics.AddError("Create connector failed", err.Error())
		return
	}

	var conn apiConnector
	if err := json.Unmarshal(respBody, &conn); err != nil {
		resp.Diagnostics.AddError("Parse error", fmt.Sprintf("Failed to parse create response: %s", err))
		return
	}

	state := connectorFromAPI(conn)

	// CreateConnector does not accept `enabled`; if the user wants disabled, PATCH immediately.
	if !plan.Enabled.IsNull() && !plan.Enabled.IsUnknown() && !plan.Enabled.ValueBool() {
		enabledFalse := false
		patchPayload := updateConnectorPayload{Name: conn.Name, Enabled: &enabledFalse}
		patchBody, _ := json.Marshal(patchPayload)
		patchURL := fmt.Sprintf("%s/connectors/%s", r.client.apiBaseURL, conn.ID)
		patchResp, _, err := doRequest(ctx, http.MethodPatch, patchURL, r.client.accessToken, patchBody)
		if err != nil {
			resp.Diagnostics.AddError("Failed to disable connector after creation", err.Error())
			return
		}
		var patched apiConnector
		if err := json.Unmarshal(patchResp, &patched); err != nil {
			resp.Diagnostics.AddError("Parse error", fmt.Sprintf("Failed to parse patch response: %s", err))
			return
		}
		state = connectorFromAPI(patched)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *connectorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state connectorModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/connectors/%s", r.client.apiBaseURL, state.ID.ValueString())

	respBody, _, err := doRequest(ctx, http.MethodGet, url, r.client.accessToken, nil)
	if err != nil {
		if errors.Is(err, errNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read connector failed", err.Error())
		return
	}

	var conn apiConnector
	if err := json.Unmarshal(respBody, &conn); err != nil {
		resp.Diagnostics.AddError("Parse error", fmt.Sprintf("Failed to parse read response: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, connectorFromAPI(conn))...)
}

func (r *connectorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan connectorModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := updateConnectorPayload{
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

	url := fmt.Sprintf("%s/connectors/%s", r.client.apiBaseURL, plan.ID.ValueString())

	respBody, _, err := doRequest(ctx, http.MethodPatch, url, r.client.accessToken, body)
	if err != nil {
		resp.Diagnostics.AddError("Update connector failed", err.Error())
		return
	}

	var conn apiConnector
	if err := json.Unmarshal(respBody, &conn); err != nil {
		resp.Diagnostics.AddError("Parse error", fmt.Sprintf("Failed to parse update response: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, connectorFromAPI(conn))...)
}

func (r *connectorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state connectorModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	url := fmt.Sprintf("%s/connectors/%s", r.client.apiBaseURL, state.ID.ValueString())

	_, _, err := doRequest(ctx, http.MethodDelete, url, r.client.accessToken, nil)
	if err != nil && !errors.Is(err, errNotFound) {
		resp.Diagnostics.AddError("Delete connector failed", err.Error())
	}
}

func (r *connectorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
