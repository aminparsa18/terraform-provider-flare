package provider

import (
	"context"

	"github.com/aminparsa18/terraform-provider-flare/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*archiveSettingsResource)(nil)
	_ resource.ResourceWithConfigure   = (*archiveSettingsResource)(nil)
	_ resource.ResourceWithImportState = (*archiveSettingsResource)(nil)
)

var archiveFormats = []string{"Parquet", "Ndjson"}

// archiveSettingsID is the fixed id of the one archive configuration a Flare instance has.
const archiveSettingsID = "archive"

// NewArchiveSettingsResource is the flare_archive_settings factory.
func NewArchiveSettingsResource() resource.Resource { return &archiveSettingsResource{} }

type archiveSettingsResource struct{ client *client.Client }

type archiveSettingsModel struct {
	ID        types.String `tfsdk:"id"`
	Enabled   types.Bool   `tfsdk:"enabled"`
	Endpoint  types.String `tfsdk:"endpoint"`
	AccessKey types.String `tfsdk:"access_key"`
	SecretKey types.String `tfsdk:"secret_key"`
	Prefix    types.String `tfsdk:"prefix"`
	Format    types.String `tfsdk:"format"`
	Signals   types.Set    `tfsdk:"signals"`
}

func (r *archiveSettingsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_archive_settings"
}

func (r *archiveSettingsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "The S3-compatible telemetry archive settings (ADR-0156, ADR-0157). A Flare instance has exactly one, so declare this resource once. " +
			"Destroying it deletes the saved settings, and the archive goes back to following the alert worker's `Archive` configuration.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true, Description: "Always `archive`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"enabled":  schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), Description: "Whether the archive runs. Defaults to true."},
			"endpoint": schema.StringAttribute{Required: true, Description: "The bucket URL, e.g. `https://s3.eu-west-1.amazonaws.com/my-bucket`."},
			"access_key": schema.StringAttribute{
				Required: true, Sensitive: true,
				Description: "S3 access key. Write-only: Flare never returns it, so changes made outside Terraform are not detected.",
			},
			"secret_key": schema.StringAttribute{
				Required: true, Sensitive: true,
				Description: "S3 secret key. Write-only, like `access_key`.",
			},
			"prefix": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("flare"), Description: "Object key prefix. Defaults to `flare`."},
			"format": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString("Parquet"),
				Description: "File format: " + joinQuoted(archiveFormats) + ". Defaults to `Parquet`.",
				Validators:  []validator.String{stringvalidator.OneOf(archiveFormats...)},
			},
			"signals": schema.SetAttribute{
				Optional: true, Computed: true, ElementType: types.StringType, Default: emptyStringSet(),
				Description: "Signals to archive: " + joinQuoted(telemetrySignals) + ". Empty (the default) archives all three.",
				Validators:  []validator.Set{setvalidator.ValueStringsAre(stringvalidator.OneOf(telemetrySignals...))},
			},
		},
	}
}

func (r *archiveSettingsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

// save is Create and Update: the API has a single PUT that creates or replaces the settings.
func (r *archiveSettingsResource) save(ctx context.Context, plan *archiveSettingsModel) (diagnosticsError string) {
	signals := []string{}
	if d := setToStrings(ctx, plan.Signals, &signals); d.HasError() {
		return d.Errors()[0].Detail()
	}
	saved, err := r.client.SaveArchiveSettings(ctx, client.ArchiveSettings{
		Enabled: boolPtr(plan.Enabled), Endpoint: plan.Endpoint.ValueString(),
		AccessKey: strPtr(plan.AccessKey), SecretKey: strPtr(plan.SecretKey),
		Prefix: strPtr(plan.Prefix), Format: strPtr(plan.Format), Signals: signals,
	})
	if err != nil {
		return err.Error()
	}
	plan.fromAPI(saved)
	return ""
}

// fromAPI folds a response into the model. Keys are left as configured: the API returns them masked.
func (m *archiveSettingsModel) fromAPI(api client.ArchiveSettings) {
	m.ID = types.StringValue(archiveSettingsID)
	m.Enabled = types.BoolValue(api.Enabled == nil || *api.Enabled)
	m.Endpoint = types.StringValue(api.Endpoint)
	m.Prefix = types.StringValue(orDefault(deref(api.Prefix), "flare"))
	m.Format = types.StringValue(orDefault(deref(api.Format), "Parquet"))
	m.Signals = stringSet(api.Signals)
}

func (r *archiveSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan archiveSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if msg := r.save(ctx, &plan); msg != "" {
		resp.Diagnostics.AddError("Saving archive settings", msg)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *archiveSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state archiveSettingsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetArchiveSettings(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Reading archive settings", err.Error())
		return
	}
	if !got.Saved {
		resp.State.RemoveResource(ctx) // reset outside Terraform: plan will save them again
		return
	}
	state.fromAPI(got)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *archiveSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan archiveSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if msg := r.save(ctx, &plan); msg != "" {
		resp.Diagnostics.AddError("Saving archive settings", msg)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *archiveSettingsResource) Delete(ctx context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	if err := r.client.DeleteArchiveSettings(ctx); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Deleting archive settings", err.Error())
	}
}

// ImportState takes any id (there is only one); the keys can't be imported, so set them in configuration.
func (r *archiveSettingsResource) ImportState(ctx context.Context, _ resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), archiveSettingsID)...)
}
