// Package provider implements the Terraform / OpenTofu provider for Flare (ADR-0146 in the Flare repo).
package provider

import (
	"context"
	"os"

	"github.com/aminparsa18/terraform-provider-flare/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// minServerVersion is the oldest Flare release whose API this provider supports: the one that added
// unique names and service-account get/delete (ADR-0146 phase 1). Update it when that release is cut;
// until then it is "0.0.0", which enforces nothing.
const minServerVersion = "0.0.0"

type flareProvider struct{ version string }

type providerModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	Token    types.String `tfsdk:"token"`
}

// New returns the provider factory.
func New(version string) func() provider.Provider {
	return func() provider.Provider { return &flareProvider{version: version} }
}

func (p *flareProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "flare"
	resp.Version = p.version
}

func (p *flareProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage Flare (self-hosted OpenTelemetry observability) configuration declaratively.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Optional:    true,
				Description: "Base URL of Flare.Api, e.g. https://flare.example.com. Defaults to the FLARE_ENDPOINT environment variable.",
			},
			"token": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Service-account access token (flr_pat_...). Defaults to the FLARE_TOKEN environment variable.",
			},
		},
	}
}

func (p *flareProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// An unknown value means it comes from another resource that isn't applied yet; configure lazily.
	if cfg.Endpoint.IsUnknown() || cfg.Token.IsUnknown() {
		return
	}

	endpoint := valueOrEnv(cfg.Endpoint, "FLARE_ENDPOINT")
	token := valueOrEnv(cfg.Token, "FLARE_TOKEN")
	if endpoint == "" {
		resp.Diagnostics.AddAttributeError(pathRoot("endpoint"), "Missing Flare endpoint", "Set the endpoint attribute or the FLARE_ENDPOINT environment variable.")
	}
	if token == "" {
		resp.Diagnostics.AddAttributeError(pathRoot("token"), "Missing Flare token", "Set the token attribute or the FLARE_TOKEN environment variable.")
	}
	if resp.Diagnostics.HasError() {
		return
	}

	c, err := client.New(endpoint, token, "terraform-provider-flare/"+p.version)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Flare configuration", err.Error())
		return
	}

	// Provider and server release separately (ADR-0068): fail early with a clear message.
	if v, err := c.Version(ctx); err != nil {
		resp.Diagnostics.AddError("Cannot reach Flare", err.Error())
		return
	} else if err := client.CheckMinimum(v.Current, minServerVersion); err != nil {
		resp.Diagnostics.AddError("Unsupported Flare version", err.Error())
		return
	}

	resp.ResourceData = c
	resp.DataSourceData = c
}

func (p *flareProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{NewNotificationChannelResource, NewAlertRuleResource, NewSLOResource, NewMaintenanceWindowResource, NewPipelineRuleResource, NewServiceAccountResource, NewIngestKeyResource, NewDashboardResource}
}

func (p *flareProvider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{NewNotificationChannelDataSource}
}

func valueOrEnv(v types.String, env string) string {
	if !v.IsNull() && v.ValueString() != "" {
		return v.ValueString()
	}
	return os.Getenv(env)
}
