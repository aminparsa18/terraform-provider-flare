package provider

import (
	"context"

	"github.com/aminparsa18/terraform-provider-flare/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*ingestKeyResource)(nil)
	_ resource.ResourceWithConfigure   = (*ingestKeyResource)(nil)
	_ resource.ResourceWithImportState = (*ingestKeyResource)(nil)
)

// NewIngestKeyResource is the flare_ingest_key factory.
func NewIngestKeyResource() resource.Resource { return &ingestKeyResource{} }

type ingestKeyResource struct{ client *client.Client }

type ingestKeyModel struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	ProjectID          types.String `tfsdk:"project_id"`
	LimitsEnabled      types.Bool   `tfsdk:"limits_enabled"`
	MaxEventsPerMinute types.Int64  `tfsdk:"max_events_per_minute"`
	MaxBytesPerMinute  types.Int64  `tfsdk:"max_bytes_per_minute"`
	MaxEventsPerDay    types.Int64  `tfsdk:"max_events_per_day"`
	MaxBytesPerDay     types.Int64  `tfsdk:"max_bytes_per_day"`
	RawKey             types.String `tfsdk:"raw_key"`
}

func (r *ingestKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ingest_key"
}

func (r *ingestKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	cap := func(desc string) schema.Int64Attribute {
		return schema.Int64Attribute{Optional: true, Description: desc + " Omit for no cap.", Validators: []validator.Int64{int64validator.AtLeast(1)}}
	}
	resp.Schema = schema.Schema{
		Description: "An OTLP ingest API key. Flare shows the secret only when the key is created, so `raw_key` is available only for keys " +
			"this provider created (it is empty after an import). Destroying the resource revokes the key, which cannot be undone.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true, Description: "Flare's id for the key.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{Required: true, Description: "Name, unique among active keys (case-insensitive). Renaming updates in place."},
			"project_id": schema.StringAttribute{
				Optional: true, Description: "Owning project id; omit for an instance-wide key.",
			},
			"limits_enabled": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				Description: "Whether the caps below are enforced. Defaults to false.",
			},
			"max_events_per_minute": cap("Events accepted per minute."),
			"max_bytes_per_minute":  cap("Bytes accepted per minute."),
			"max_events_per_day":    cap("Events accepted per UTC day."),
			"max_bytes_per_day":     cap("Bytes accepted per UTC day."),
			"raw_key": schema.StringAttribute{
				Computed: true, Sensitive: true,
				Description:   "The secret to configure in OTLP exporters (Authorization / x-api-key). Only known for keys created by this resource.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *ingestKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func int64Ptr(v types.Int64) *int64 {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	i := v.ValueInt64()
	return &i
}

func int64Value(p *int64) types.Int64 {
	if p == nil {
		return types.Int64Null()
	}
	return types.Int64Value(*p)
}

func (m ingestKeyModel) limits() client.IngestKeyLimits {
	return client.IngestKeyLimits{
		LimitsEnabled:      m.LimitsEnabled.ValueBool(),
		MaxEventsPerMinute: int64Ptr(m.MaxEventsPerMinute), MaxBytesPerMinute: int64Ptr(m.MaxBytesPerMinute),
		MaxEventsPerDay: int64Ptr(m.MaxEventsPerDay), MaxBytesPerDay: int64Ptr(m.MaxBytesPerDay),
	}
}

// projectPtr treats null and "" as instance-wide.
func (m ingestKeyModel) projectPtr() *string {
	if p := strPtr(m.ProjectID); p != nil && *p != "" {
		return p
	}
	return nil
}

// fromAPI leaves RawKey alone: it only exists on the create response.
func (m *ingestKeyModel) fromAPI(api client.IngestKey) {
	m.ID = types.StringValue(api.ID)
	m.Name = types.StringValue(api.Name)
	m.ProjectID = readString(api.ProjectID, m.ProjectID)
	if api.ProjectID != nil && *api.ProjectID == emptyGUID {
		m.ProjectID = types.StringNull()
	}
	m.LimitsEnabled = types.BoolValue(api.LimitsEnabled)
	m.MaxEventsPerMinute, m.MaxBytesPerMinute = int64Value(api.MaxEventsPerMinute), int64Value(api.MaxBytesPerMinute)
	m.MaxEventsPerDay, m.MaxBytesPerDay = int64Value(api.MaxEventsPerDay), int64Value(api.MaxBytesPerDay)
}

const emptyGUID = "00000000-0000-0000-0000-000000000000"

func (m ingestKeyModel) hasLimits() bool {
	l := m.limits()
	return l.LimitsEnabled || l.MaxEventsPerMinute != nil || l.MaxBytesPerMinute != nil || l.MaxEventsPerDay != nil || l.MaxBytesPerDay != nil
}

func (r *ingestKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ingestKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	key, raw, err := r.client.CreateIngestKey(ctx, plan.Name.ValueString(), plan.projectPtr())
	if err != nil {
		resp.Diagnostics.AddError("Creating ingest key", err.Error())
		return
	}
	// Record the key before the follow-up call so a failure there still leaves the (secret-bearing) key in state.
	plan.ID, plan.RawKey = types.StringValue(key.ID), types.StringValue(raw)
	if plan.hasLimits() {
		if err := r.client.SetIngestKeyLimits(ctx, key.ID, plan.limits()); err != nil {
			resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
			resp.Diagnostics.AddError("Setting ingest key limits", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *ingestKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ingestKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetIngestKey(ctx, state.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx) // revoked outside Terraform
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Reading ingest key", err.Error())
		return
	}
	state.fromAPI(got)
	if state.RawKey.IsNull() || state.RawKey.IsUnknown() {
		state.RawKey = types.StringValue("") // imported: Flare no longer knows the secret
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *ingestKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state ingestKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	steps := []struct {
		what string
		do   func() error
	}{
		{"Renaming ingest key", func() error { return r.client.RenameIngestKey(ctx, id, plan.Name.ValueString()) }},
		{"Setting ingest key project", func() error { return r.client.SetIngestKeyProject(ctx, id, plan.projectPtr()) }},
		{"Setting ingest key limits", func() error { return r.client.SetIngestKeyLimits(ctx, id, plan.limits()) }},
	}
	for i, s := range steps {
		changed := []bool{
			!plan.Name.Equal(state.Name),
			!plan.ProjectID.Equal(state.ProjectID),
			!plan.LimitsEnabled.Equal(state.LimitsEnabled) || !plan.MaxEventsPerMinute.Equal(state.MaxEventsPerMinute) ||
				!plan.MaxBytesPerMinute.Equal(state.MaxBytesPerMinute) || !plan.MaxEventsPerDay.Equal(state.MaxEventsPerDay) ||
				!plan.MaxBytesPerDay.Equal(state.MaxBytesPerDay),
		}[i]
		if !changed {
			continue
		}
		if err := s.do(); err != nil {
			resp.Diagnostics.AddError(s.what, err.Error())
			return
		}
	}
	plan.RawKey = state.RawKey
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *ingestKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ingestKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.RevokeIngestKey(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Revoking ingest key", err.Error())
	}
}

// ImportState accepts an active key's id or name.
func (r *ingestKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := req.ID
	if !guidPattern.MatchString(id) {
		found, err := r.client.FindIngestKeyByName(ctx, id)
		if err != nil {
			resp.Diagnostics.AddError("Importing ingest key", err.Error())
			return
		}
		id = found.ID
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}
