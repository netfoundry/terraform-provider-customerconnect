package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ provider.Provider = &customerConnectProvider{}

// authURLs maps deploy environment names to their Cognito OAuth2 token endpoints.
var authURLs = map[string]string{
	"sandbox":    "https://netfoundry-sandbox-hnssty.auth.us-east-1.amazoncognito.com/oauth2/token",
	"staging":    "https://netfoundry-staging-mlvyyc.auth.us-east-1.amazoncognito.com/oauth2/token",
	"production": "https://netfoundry-production-xfjiye.auth.us-east-1.amazoncognito.com/oauth2/token",
}

type customerConnectProvider struct {
	version string
}

// customerConnectData is passed to every resource and data source via resp.ResourceData / resp.DataSourceData.
type customerConnectData struct {
	accessToken string
	environment string
	apiBaseURL  string
}

type customerConnectProviderModel struct {
	ClientID     types.String `tfsdk:"client_id"`
	ClientSecret types.String `tfsdk:"client_secret"`
	Environment  types.String `tfsdk:"environment"`
	AuthURL      types.String `tfsdk:"auth_url"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &customerConnectProvider{version: version}
	}
}

func (p *customerConnectProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "customer-connect"
	resp.Version = p.version
}

func (p *customerConnectProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "NetFoundry Customer Connect Terraform Provider",
		Attributes: map[string]schema.Attribute{
			"client_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Cognito OAuth2 client ID for the NetFoundry API. Env: NF_CLIENT_ID.",
			},
			"client_secret": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Cognito OAuth2 client secret for the NetFoundry API. Env: NF_CLIENT_SECRET.",
			},
			"environment": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "NetFoundry deploy environment: `sandbox`, `staging`, or `production` (default). Env: NF_ENVIRONMENT.",
			},
			"auth_url": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Override the Cognito OAuth2 token endpoint. Inferred from `environment` when omitted.",
			},
		},
	}
}

// fetchAccessToken performs the Cognito OAuth2 client_credentials grant against the Cognito endpoint.
// It replicates:
//
//	curl --user <client_id>:<client_secret> --request POST <authURL> \
//	  --header 'content-type: application/x-www-form-urlencoded' \
//	  --data 'grant_type=client_credentials&scope=https://gateway.<env>.netfoundry.io//ignore-scope'
func fetchAccessToken(authURL, clientID, clientSecret, environment string) (string, error) {
	scope := fmt.Sprintf("https://gateway.%s.netfoundry.io//ignore-scope", environment)
	formData := url.Values{}
	formData.Set("grant_type", "client_credentials")
	formData.Set("scope", scope)

	httpClient := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("POST", authURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to build auth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("auth request failed: %w", err)
	}
	body, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return "", fmt.Errorf("error reading auth response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("auth returned status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse auth response: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("no access_token in auth response")
	}
	return tokenResp.AccessToken, nil
}

func (p *customerConnectProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	tflog.Info(ctx, "Configuring customer-connect client")

	var config customerConnectProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	for _, attr := range []struct {
		unknown bool
		name    string
	}{
		{config.ClientID.IsUnknown(), "client_id"},
		{config.ClientSecret.IsUnknown(), "client_secret"},
		{config.Environment.IsUnknown(), "environment"},
		{config.AuthURL.IsUnknown(), "auth_url"},
	} {
		if attr.unknown {
			resp.Diagnostics.AddAttributeError(
				path.Root(attr.name),
				fmt.Sprintf("Unknown %s", attr.name),
				"Either target apply the source of the value first or set the value statically in the configuration.",
			)
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}

	// Resolve client_id (config > env).
	clientID := os.Getenv("NF_CLIENT_ID")
	if !config.ClientID.IsNull() {
		clientID = config.ClientID.ValueString()
	}

	// Resolve client_secret (config > env).
	clientSecret := os.Getenv("NF_CLIENT_SECRET")
	if !config.ClientSecret.IsNull() {
		clientSecret = config.ClientSecret.ValueString()
	}

	// Resolve environment (config > env > default "production").
	environment := os.Getenv("NF_ENVIRONMENT")
	if environment == "" {
		environment = "production"
	}
	if !config.Environment.IsNull() {
		environment = config.Environment.ValueString()
	}

	// Validate environment.
	if _, ok := authURLs[environment]; !ok {
		resp.Diagnostics.AddAttributeError(
			path.Root("environment"),
			"Invalid environment",
			fmt.Sprintf("Must be one of: sandbox, staging, production. Got: %q", environment),
		)
		return
	}

	// Resolve auth URL (explicit override > environment default).
	authURL := authURLs[environment]
	if !config.AuthURL.IsNull() {
		authURL = config.AuthURL.ValueString()
	}

	if clientID == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("client_id"),
			"Missing client_id",
			"Provide 'client_id' in the provider configuration or set NF_CLIENT_ID.",
		)
	}
	if clientSecret == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("client_secret"),
			"Missing client_secret",
			"Provide 'client_secret' in the provider configuration or set NF_CLIENT_SECRET.",
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Fetching NetFoundry access token", map[string]any{
		"environment": environment,
		"auth_url":    authURL,
	})

	accessToken, err := fetchAccessToken(authURL, clientID, clientSecret, environment)
	if err != nil {
		resp.Diagnostics.AddError("Authentication failed", err.Error())
		return
	}

	data := &customerConnectData{
		accessToken: accessToken,
		environment: environment,
		apiBaseURL:  fmt.Sprintf("https://gateway.%s.netfoundry.io/customer-connect/v1", environment),
	}
	resp.DataSourceData = data
	resp.ResourceData = data

	tflog.Info(ctx, "Configured customer-connect client", map[string]any{
		"environment": environment,
		"success":     true,
	})
}

func (p *customerConnectProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewLocationDataSource,
	}
}

func (p *customerConnectProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewLocationResource,
	}
}
