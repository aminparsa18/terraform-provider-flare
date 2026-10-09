package provider

import (
	"context"

	"github.com/aminparsa18/terraform-provider-flare/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*forwardingTargetResource)(nil)
	_ resource.ResourceWithConfigure   = (*forwardingTargetResource)(nil)
	_ resource.ResourceWithImportState = (*forwardingTargetResource)(nil)
)

var telemetrySignals = []string{"Logs", "Traces", "Metrics"}

// NewForwardingTargetResource is the flare_forwarding_target factory.
func NewForwardingTargetResource() resource.Resource { return &forwardingTargetResource{} }

type forwardingTargetResource struct{ client *client.Client }

type forwardingTargetModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Enabled      types.Bool   `tfsdk:"enabled"`
	Endpoint     types.String `tfsdk:"endpoint"`
	Headers      types.Map    `tfsdk:"headers"`
	Signals      types.Set    `tfsdk:"signals"`
	Services     types.Set    `tfsdk:"services"`
	IngestKeyIDs types.Set    `tfsdk:"ingest_key_ids"`
	Gzip         types.Bool   `tfsdk:"gzip"`
}

func emptyStringSet() defaults.Set {
	return setdefault.StaticValue(types.SetValueMust(types.StringType, []attr.Value{}))
}

func (r *forwardingTargetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_forwarding_target"
}

func (r *forwardingTargetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A managed OTLP forwarding target: Flare copies the telemetry it ingests to another OTLP/HTTP endpoint (ADR-0155, ADR-0157). " +
			"Targets defined in the server's `Forwarding:Targets` configuration are separate and not managed here.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true, Description: "Flare's id for the target.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name":    schema.StringAttribute{Required: true, Description: "Unique (case-insensitive) name. Renaming updates the target in place."},
			"enabled": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), Description: "Whether the target is forwarding. A disabled target has its queue dropped. Defaults to true."},
			"endpoint": schema.StringAttribute{
				Required: true, Description: "The destination's absolute OTLP/HTTP base URL (http or https).",
			},
			"headers": schema.MapAttribute{
				Optional: true, Sensitive: true, ElementType: types.StringType,
				Description: "Request headers sent with every forwarded request, typically credentials. Write-only: Flare never returns the values, so changes made outside Terraform are not detected.",
			},
			"signals": schema.SetAttribute{
				Optional: true, Computed: true, ElementType: types.StringType, Default: emptyStringSet(),
				Description: "Signals to forward: " + joinQuoted(telemetrySignals) + ". Empty (the default) forwards all three.",
				Validators:  []validator.Set{setvalidator.ValueStringsAre(stringvalidator.OneOf(telemetrySignals...))},
			},
			"services": schema.SetAttribute{
				Optional: true, Computed: true, ElementType: types.StringType, Default: emptyStringSet(),
				Description: "Forward only these services' telemetry. Empty (the default) forwards every service.",
			},
			"ingest_key_ids": schema.SetAttribute{
				Optional: true, Computed: true, ElementType: types.StringType, Default: emptyStringSet(),
				Description: "Forward only requests that arrived with one of these ingest keys (ids, e.g. `flare_ingest_key.x.id`). Empty (the default) forwards any request.",
			},
			"gzip": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), Description: "Compress forwarded request bodies. Defaults to true."},
		},
	}
}

func (r *forwardingTargetResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (m forwardingTargetModel) toAPI(ctx context.Context) (client.ForwardingTarget, diag.Diagnostics) {
	headers := map[string]string{}
	signals, services, keys := []string{}, []string{}, []string{}
	var diags diag.Diagnostics
	if !m.Headers.IsNull() && !m.Headers.IsUnknown() {
		diags.Append(m.Headers.ElementsAs(ctx, &headers, false)...)
	}
	diags.Append(setToStrings(ctx, m.Signals, &signals)...)
	diags.Append(setToStrings(ctx, m.Services, &services)...)
	diags.Append(setToStrings(ctx, m.IngestKeyIDs, &keys)...)
	return client.ForwardingTarget{
		Name: m.Name.ValueString(), Enabled: boolPtr(m.Enabled), Endpoint: m.Endpoint.ValueString(), Headers: headers,
		Signals: signals, Services: services, IngestKeyIDs: keys, Gzip: boolPtr(m.Gzip),
	}, diags
}

// fromAPI folds a response into the model. Headers are left as configured: the API returns them masked.
func (m *forwardingTargetModel) fromAPI(api client.ForwardingTarget) {
	m.ID = types.StringValue(api.ID)
	m.Name = types.StringValue(api.Name)
	m.Enabled = types.BoolValue(api.Enabled == nil || *api.Enabled)
	m.Endpoint = types.StringValue(api.Endpoint)
	m.Gzip = types.BoolValue(api.Gzip == nil || *api.Gzip)
	m.Signals = stringSet(api.Signals)
	m.Services = stringSet(api.Services)
	m.IngestKeyIDs = stringSet(api.IngestKeyIDs)
}

func (r *forwardingTargetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan forwardingTargetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body, diags := plan.toAPI(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	created, err := r.client.CreateForwardingTarget(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Creating forwarding target", err.Error())
		return
	}
	plan.fromAPI(created)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *forwardingTargetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state forwardingTargetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetForwardingTarget(ctx, state.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Reading forwarding target", err.Error())
		return
	}
	state.fromAPI(got)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *forwardingTargetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state forwardingTargetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body, diags := plan.toAPI(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	updated, err := r.client.UpdateForwardingTarget(ctx, state.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Updating forwarding target", err.Error())
		return
	}
	plan.fromAPI(updated)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *forwardingTargetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state forwardingTargetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteForwardingTarget(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Deleting forwarding target", err.Error())
	}
}

// ImportState accepts a target id or its name. Header values can't be imported (Flare never returns them);
// set them in configuration and the next apply writes them.
func (r *forwardingTargetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := req.ID
	if !guidPattern.MatchString(id) {
		found, err := r.client.FindForwardingTargetByName(ctx, id)
		if err != nil {
			resp.Diagnostics.AddError("Importing forwarding target", err.Error())
			return
		}
		id = found.ID
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}
