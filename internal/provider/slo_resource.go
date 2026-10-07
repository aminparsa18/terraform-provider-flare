package provider

import (
	"context"

	"github.com/aminparsa18/terraform-provider-flare/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*sloResource)(nil)
	_ resource.ResourceWithConfigure   = (*sloResource)(nil)
	_ resource.ResourceWithImportState = (*sloResource)(nil)
)

var sloKinds = []string{"Availability", "Latency"}

// NewSLOResource is the flare_slo factory.
func NewSLOResource() resource.Resource { return &sloResource{} }

type sloResource struct{ client *client.Client }

type sloModel struct {
	ID                 types.String  `tfsdk:"id"`
	Name               types.String  `tfsdk:"name"`
	Description        types.String  `tfsdk:"description"`
	Kind               types.String  `tfsdk:"kind"`
	ServiceName        types.String  `tfsdk:"service_name"`
	OperationName      types.String  `tfsdk:"operation_name"`
	TargetPercent      types.Float64 `tfsdk:"target_percent"`
	LatencyThresholdMs types.Int64   `tfsdk:"latency_threshold_ms"`
	WindowDays         types.Int64   `tfsdk:"window_days"`
}

func (r *sloResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_slo"
}

func (r *sloResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A service-level objective over a service's (optionally one operation's) entry spans. Reference `flare_slo.x.id` from a `flare_alert_rule`'s `slo_condition` for burn-rate alerts.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true, Description: "Flare's id for the SLO.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{Required: true, Description: "Unique (case-insensitive) name, at most 200 characters. Renaming updates in place."},
			"description": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Free-text description.",
			},
			"kind": schema.StringAttribute{
				Required: true, Description: "One of: " + joinQuoted(sloKinds) + ".",
				Validators: []validator.String{stringvalidator.OneOf(sloKinds...)},
			},
			"service_name": schema.StringAttribute{Required: true, Description: "Service whose spans are measured."},
			"operation_name": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString(""),
				Description: "Restrict to one operation (span name); empty means the whole service.",
			},
			"target_percent": schema.Float64Attribute{
				Required: true, Description: "Objective, from 1 to 99.999.",
				Validators: []validator.Float64{float64validator.Between(1, 99.999)},
			},
			"latency_threshold_ms": schema.Int64Attribute{
				Optional: true, Computed: true, Default: int64default.StaticInt64(0),
				Description: "Latency SLOs only: a span slower than this is bad. Must be one of Flare's fixed thresholds; Flare rejects others.",
				Validators:  []validator.Int64{int64validator.AtLeast(0)},
			},
			"window_days": schema.Int64Attribute{
				Optional: true, Computed: true, Default: int64default.StaticInt64(30),
				Description: "Rolling compliance window in days, 1 to 90. Defaults to 30.",
				Validators:  []validator.Int64{int64validator.Between(1, 90)},
			},
		},
	}
}

func (r *sloResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (m sloModel) toAPI() client.SLO {
	out := client.SLO{
		Name: m.Name.ValueString(), Description: strPtr(m.Description), Kind: m.Kind.ValueString(),
		ServiceName: m.ServiceName.ValueString(), OperationName: strPtr(m.OperationName),
		TargetPercent: m.TargetPercent.ValueFloat64(), WindowDays: intPtr(m.WindowDays),
	}
	// The API rejects a latency threshold on an availability SLO, even 0 is fine but omit it anyway.
	if m.Kind.ValueString() == "Latency" {
		out.LatencyThresholdMs = intPtr(m.LatencyThresholdMs)
	}
	return out
}

func (m *sloModel) fromAPI(api client.SLO) {
	m.ID = types.StringValue(api.ID)
	m.Name = types.StringValue(api.Name)
	m.Description = types.StringValue(deref(api.Description))
	m.Kind = types.StringValue(api.Kind)
	m.ServiceName = types.StringValue(api.ServiceName)
	m.OperationName = types.StringValue(deref(api.OperationName))
	m.TargetPercent = types.Float64Value(api.TargetPercent)
	m.LatencyThresholdMs = types.Int64Value(int64(derefInt(api.LatencyThresholdMs, 0)))
	m.WindowDays = types.Int64Value(int64(derefInt(api.WindowDays, 30)))
}

func (r *sloResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan sloModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	created, err := r.client.CreateSLO(ctx, plan.toAPI())
	if err != nil {
		resp.Diagnostics.AddError("Creating SLO", err.Error())
		return
	}
	plan.fromAPI(created)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *sloResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state sloModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetSLO(ctx, state.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Reading SLO", err.Error())
		return
	}
	state.fromAPI(got)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *sloResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state sloModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	updated, err := r.client.UpdateSLO(ctx, state.ID.ValueString(), plan.toAPI())
	if err != nil {
		resp.Diagnostics.AddError("Updating SLO", err.Error())
		return
	}
	plan.fromAPI(updated)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *sloResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state sloModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteSLO(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Deleting SLO", err.Error())
	}
}

// ImportState accepts an SLO id or its name.
func (r *sloResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := req.ID
	if !guidPattern.MatchString(id) {
		found, err := r.client.FindSLOByName(ctx, id)
		if err != nil {
			resp.Diagnostics.AddError("Importing SLO", err.Error())
			return
		}
		id = found.ID
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}
