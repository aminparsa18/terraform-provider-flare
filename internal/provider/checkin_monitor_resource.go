package provider

import (
	"context"

	"github.com/aminparsa18/terraform-provider-flare/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                     = (*checkInMonitorResource)(nil)
	_ resource.ResourceWithConfigure        = (*checkInMonitorResource)(nil)
	_ resource.ResourceWithImportState      = (*checkInMonitorResource)(nil)
	_ resource.ResourceWithConfigValidators = (*checkInMonitorResource)(nil)
)

// NewCheckInMonitorResource is the flare_checkin_monitor factory.
func NewCheckInMonitorResource() resource.Resource { return &checkInMonitorResource{} }

type checkInMonitorResource struct{ client *client.Client }

type checkInMonitorModel struct {
	ID                types.String `tfsdk:"id"`
	Slug              types.String `tfsdk:"slug"`
	Name              types.String `tfsdk:"name"`
	Description       types.String `tfsdk:"description"`
	Enabled           types.Bool   `tfsdk:"enabled"`
	CronExpression    types.String `tfsdk:"cron_expression"`
	IntervalSeconds   types.Int64  `tfsdk:"interval_seconds"`
	TimeZone          types.String `tfsdk:"time_zone"`
	GraceSeconds      types.Int64  `tfsdk:"grace_seconds"`
	MaxRuntimeSeconds types.Int64  `tfsdk:"max_runtime_seconds"`
}

func (r *checkInMonitorResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_checkin_monitor"
}

func (r *checkInMonitorResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A cron or job check-in monitor: a schedule Flare expects a job to ping (`/v1/checkins/{slug}` on the ingest port). " +
			"Flare writes the `checkin.up` gauge for it, so an alert is an ordinary `flare_alert_rule` metric rule on `checkin.up` " +
			"with `Last` below 1. Set exactly one of `cron_expression` and `interval_seconds`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true, Description: "Flare's id for the monitor.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"slug": schema.StringAttribute{
				Optional: true, Computed: true,
				Description: "The name a job puts in the ping URL: 1-64 lower-case letters, digits, `-` or `_`. Derived from `name` when omitted. " +
					"Fixed after creation, so changing it replaces the monitor.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{Required: true, Description: "Display name, at most 200 characters. Renaming updates in place."},
			"description": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Free-text description.",
			},
			"enabled": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(true),
				Description: "Whether Flare judges the monitor. A paused monitor writes no `checkin.up` point. Defaults to `true`.",
			},
			"cron_expression": schema.StringAttribute{
				Optional:    true,
				Description: "A five-field cron expression (`minute hour day-of-month month day-of-week`) or a macro such as `@daily`, read in `time_zone`.",
			},
			"interval_seconds": schema.Int64Attribute{
				Optional:    true,
				Description: "A fixed period between pings, 60 to 2678400 (31 days). The next ping is measured from the last one.",
				Validators:  []validator.Int64{int64validator.Between(60, 2678400)},
			},
			"time_zone": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString("UTC"),
				Description: "IANA time zone `cron_expression` is read in. Defaults to `UTC`.",
			},
			"grace_seconds": schema.Int64Attribute{
				Optional: true, Computed: true, Default: int64default.StaticInt64(300),
				Description: "How long after the expected time a ping may arrive before the job counts as late, 0 to 604800. Defaults to 300.",
				Validators:  []validator.Int64{int64validator.Between(0, 604800)},
			},
			"max_runtime_seconds": schema.Int64Attribute{
				Optional: true, Computed: true, Default: int64default.StaticInt64(0),
				Description: "A run that has started and not finished within this many seconds counts as overran, 0 to 604800. 0 (the default) means no limit.",
				Validators:  []validator.Int64{int64validator.Between(0, 604800)},
			},
		},
	}
}

func (r *checkInMonitorResource) ConfigValidators(context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(path.MatchRoot("cron_expression"), path.MatchRoot("interval_seconds")),
	}
}

func (r *checkInMonitorResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (m checkInMonitorModel) toAPI() client.CheckInMonitor {
	return client.CheckInMonitor{
		Slug: m.Slug.ValueString(), Name: m.Name.ValueString(), Description: strPtr(m.Description), Enabled: boolPtr(m.Enabled),
		CronExpression: strPtr(m.CronExpression), IntervalSeconds: intPtr(m.IntervalSeconds), TimeZone: strPtr(m.TimeZone),
		GraceSeconds: intPtr(m.GraceSeconds), MaxRuntimeSeconds: intPtr(m.MaxRuntimeSeconds),
	}
}

func (m *checkInMonitorModel) fromAPI(api client.CheckInMonitor) {
	m.ID = types.StringValue(api.ID)
	m.Slug = types.StringValue(api.Slug)
	m.Name = types.StringValue(api.Name)
	m.Description = types.StringValue(deref(api.Description))
	m.Enabled = types.BoolValue(api.Enabled == nil || *api.Enabled)
	// The API reports "" / 0 for the schedule kind that is not in use; that must read back as null.
	if c := deref(api.CronExpression); c != "" {
		m.CronExpression = types.StringValue(c)
	} else {
		m.CronExpression = types.StringNull()
	}
	if i := derefInt(api.IntervalSeconds, 0); i > 0 {
		m.IntervalSeconds = types.Int64Value(int64(i))
	} else {
		m.IntervalSeconds = types.Int64Null()
	}
	m.TimeZone = types.StringValue(orDefault(deref(api.TimeZone), "UTC"))
	m.GraceSeconds = types.Int64Value(int64(derefInt(api.GraceSeconds, 300)))
	m.MaxRuntimeSeconds = types.Int64Value(int64(derefInt(api.MaxRuntimeSeconds, 0)))
}

func (r *checkInMonitorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan checkInMonitorModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := plan.toAPI()
	if plan.Slug.IsUnknown() {
		in.Slug = "" // the API derives it from the name
	}
	created, err := r.client.CreateCheckInMonitor(ctx, in)
	if err != nil {
		resp.Diagnostics.AddError("Creating check-in monitor", err.Error())
		return
	}
	plan.fromAPI(created)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *checkInMonitorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state checkInMonitorModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetCheckInMonitor(ctx, state.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Reading check-in monitor", err.Error())
		return
	}
	state.fromAPI(got)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *checkInMonitorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state checkInMonitorModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	updated, err := r.client.UpdateCheckInMonitor(ctx, state.ID.ValueString(), plan.toAPI())
	if err != nil {
		resp.Diagnostics.AddError("Updating check-in monitor", err.Error())
		return
	}
	plan.fromAPI(updated)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *checkInMonitorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state checkInMonitorModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteCheckInMonitor(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Deleting check-in monitor", err.Error())
	}
}

// ImportState accepts a monitor id or its slug.
func (r *checkInMonitorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := req.ID
	if !guidPattern.MatchString(id) {
		found, err := r.client.FindCheckInMonitorBySlug(ctx, id)
		if err != nil {
			resp.Diagnostics.AddError("Importing check-in monitor", err.Error())
			return
		}
		id = found.ID
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}
