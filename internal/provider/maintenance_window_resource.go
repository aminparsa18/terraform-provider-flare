package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/aminparsa18/terraform-provider-flare/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*maintenanceWindowResource)(nil)
	_ resource.ResourceWithConfigure   = (*maintenanceWindowResource)(nil)
	_ resource.ResourceWithImportState = (*maintenanceWindowResource)(nil)
)

var (
	maintenanceRecurrences = []string{"None", "Daily", "Weekly"}
	weekdays               = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
)

// NewMaintenanceWindowResource is the flare_maintenance_window factory.
func NewMaintenanceWindowResource() resource.Resource { return &maintenanceWindowResource{} }

type maintenanceWindowResource struct{ client *client.Client }

type maintenanceWindowModel struct {
	ID            types.String            `tfsdk:"id"`
	Name          types.String            `tfsdk:"name"`
	Description   types.String            `tfsdk:"description"`
	RuleNames     []types.String          `tfsdk:"rule_names"`
	StartsAt      types.String            `tfsdk:"starts_at"`
	EndsAt        types.String            `tfsdk:"ends_at"`
	Recurrence    types.String            `tfsdk:"recurrence"`
	DaysOfWeek    types.Set               `tfsdk:"days_of_week"`
	RepeatUntil   types.String            `tfsdk:"repeat_until"`
	TimeZone      types.String            `tfsdk:"time_zone"`
	LabelMatchers map[string]types.String `tfsdk:"label_matchers"`
}

func (r *maintenanceWindowResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_maintenance_window"
}

func (r *maintenanceWindowResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A maintenance window: while one is active, alert rules it covers still evaluate but a breach is recorded as " +
			"suppressed instead of notifying. With neither `rule_names` nor `label_matchers` it covers every rule; with either, " +
			"it covers a rule that is listed or whose labels match.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true, Description: "Flare's id for the window.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{Required: true, Description: "Unique (case-insensitive) name, at most 200 characters. Renaming updates in place."},
			"description": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Free-text description.",
			},
			"rule_names": schema.ListAttribute{
				Optional: true, ElementType: types.StringType,
				Description: "Names of `flare_alert_rule`s this window silences, resolved to ids at apply time.",
			},
			"starts_at": schema.StringAttribute{Required: true, Description: "RFC 3339 start of the first (or only) occurrence, e.g. `2026-11-01T02:00:00Z`."},
			"ends_at": schema.StringAttribute{
				Required:    true,
				Description: "RFC 3339 end of the first occurrence; every occurrence of a recurring window lasts `ends_at - starts_at` (at most 24 hours for `Daily`, 7 days for `Weekly`).",
			},
			"recurrence": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString("None"),
				Description: "One of: " + joinQuoted(maintenanceRecurrences) + ". Defaults to `None` (a one-off window).",
				Validators:  []validator.String{stringvalidator.OneOf(maintenanceRecurrences...)},
			},
			"days_of_week": schema.SetAttribute{
				Optional: true, ElementType: types.StringType,
				Description: "`Weekly` only: the local weekdays an occurrence starts on, from " + joinQuoted(weekdays) + ".",
				Validators:  []validator.Set{setvalidator.ValueStringsAre(stringvalidator.OneOf(weekdays...))},
			},
			"repeat_until": schema.StringAttribute{Optional: true, Description: "Recurring windows only: RFC 3339 instant after which no occurrence starts. Omit to repeat forever."},
			"time_zone": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString("UTC"),
				Description: "IANA time zone a recurring window's time of day and weekdays are computed in. Defaults to `UTC`.",
			},
			"label_matchers": schema.MapAttribute{
				Optional: true, ElementType: types.StringType,
				Description: "Label key to value pairs a rule's labels must all contain for the window to cover it, so rules created later are covered automatically.",
			},
		},
	}
}

func (r *maintenanceWindowResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

// ruleDirectory resolves alert-rule names to ids and back using one list call.
type ruleDirectory struct {
	byName map[string]string
	byID   map[string]string
}

func (r *maintenanceWindowResource) rules(ctx context.Context) (ruleDirectory, error) {
	all, err := r.client.ListAlertRules(ctx)
	if err != nil {
		return ruleDirectory{}, err
	}
	d := ruleDirectory{byName: map[string]string{}, byID: map[string]string{}}
	for _, rule := range all {
		d.byName[lower(rule.Name)] = rule.ID
		d.byID[rule.ID] = rule.Name
	}
	return d, nil
}

func (d ruleDirectory) ids(names []types.String) ([]string, error) {
	var out []string
	for _, n := range names {
		id, ok := d.byName[lower(n.ValueString())]
		if !ok {
			return nil, fmt.Errorf("no alert rule named %q", n.ValueString())
		}
		out = append(out, id)
	}
	return out, nil
}

func (d ruleDirectory) names(ids []string, prior []types.String) []types.String {
	var names []string
	for _, id := range ids {
		if n, ok := d.byID[id]; ok {
			names = append(names, n)
		}
		// A rule deleted out from under the window drops out of state, so the plan re-adds the reference.
	}
	return stringValues(names, prior)
}

// sameInstant keeps the configured spelling of a timestamp when the API's normalised one is the same
// instant, so `2026-11-01T02:00:00Z` doesn't diff against `2026-11-01T02:00:00+00:00`.
func sameInstant(api string, prior types.String) types.String {
	if !prior.IsNull() && !prior.IsUnknown() {
		a, errA := time.Parse(time.RFC3339, api)
		p, errP := time.Parse(time.RFC3339, prior.ValueString())
		if errA == nil && errP == nil && a.Equal(p) {
			return prior
		}
	}
	// No (or a different) prior value, e.g. after import: normalise to UTC so the common `Z` spelling matches.
	if a, err := time.Parse(time.RFC3339, api); err == nil {
		return types.StringValue(a.UTC().Format(time.RFC3339))
	}
	return types.StringValue(api)
}

func (m maintenanceWindowModel) toAPI(dir ruleDirectory) (client.MaintenanceWindow, error) {
	out := client.MaintenanceWindow{
		Name: m.Name.ValueString(), Description: strPtr(m.Description),
		StartsAt: m.StartsAt.ValueString(), EndsAt: m.EndsAt.ValueString(),
		Recurrence: m.Recurrence.ValueString(), RepeatUntil: strPtr(m.RepeatUntil), TimeZone: strPtr(m.TimeZone),
	}
	for _, f := range []struct{ name, v string }{{"starts_at", out.StartsAt}, {"ends_at", out.EndsAt}} {
		if _, err := time.Parse(time.RFC3339, f.v); err != nil {
			return out, fmt.Errorf("%s: %q is not an RFC 3339 timestamp", f.name, f.v)
		}
	}
	if out.RepeatUntil != nil {
		if _, err := time.Parse(time.RFC3339, *out.RepeatUntil); err != nil {
			return out, fmt.Errorf("repeat_until: %q is not an RFC 3339 timestamp", *out.RepeatUntil)
		}
	}
	var err error
	if out.RuleIDs, err = dir.ids(m.RuleNames); err != nil {
		return out, fmt.Errorf("rule_names: %w", err)
	}
	if !m.DaysOfWeek.IsNull() && !m.DaysOfWeek.IsUnknown() {
		var days []types.String
		m.DaysOfWeek.ElementsAs(context.Background(), &days, false)
		out.DaysOfWeek = stringSlice(days)
	}
	if m.LabelMatchers != nil {
		out.LabelMatchers = map[string]string{}
		for k, v := range m.LabelMatchers {
			out.LabelMatchers[k] = v.ValueString()
		}
	}
	return out, nil
}

func (m *maintenanceWindowModel) fromAPI(api client.MaintenanceWindow, dir ruleDirectory) {
	m.ID = types.StringValue(api.ID)
	m.Name = types.StringValue(api.Name)
	m.Description = types.StringValue(deref(api.Description))
	m.RuleNames = dir.names(api.RuleIDs, m.RuleNames)
	m.StartsAt = sameInstant(api.StartsAt, m.StartsAt)
	m.EndsAt = sameInstant(api.EndsAt, m.EndsAt)
	m.Recurrence = types.StringValue(orDefault(api.Recurrence, "None"))
	m.TimeZone = types.StringValue(orDefault(deref(api.TimeZone), "UTC"))
	if api.RepeatUntil != nil && *api.RepeatUntil != "" {
		m.RepeatUntil = sameInstant(*api.RepeatUntil, m.RepeatUntil)
	} else {
		m.RepeatUntil = types.StringNull()
	}

	switch {
	case len(api.DaysOfWeek) > 0:
		m.DaysOfWeek, _ = types.SetValueFrom(context.Background(), types.StringType, api.DaysOfWeek)
	case !m.DaysOfWeek.IsNull() && !m.DaysOfWeek.IsUnknown():
		m.DaysOfWeek, _ = types.SetValueFrom(context.Background(), types.StringType, []string{})
	default:
		m.DaysOfWeek = types.SetNull(types.StringType)
	}

	switch {
	case len(api.LabelMatchers) > 0:
		m.LabelMatchers = make(map[string]types.String, len(api.LabelMatchers))
		for k, v := range api.LabelMatchers {
			m.LabelMatchers[k] = types.StringValue(v)
		}
	case m.LabelMatchers != nil:
		m.LabelMatchers = map[string]types.String{}
	}
}

func (r *maintenanceWindowResource) build(ctx context.Context, m maintenanceWindowModel, diags *diag.Diagnostics) (client.MaintenanceWindow, ruleDirectory, bool) {
	dir, err := r.rules(ctx)
	if err != nil {
		diags.AddError("Listing alert rules", err.Error())
		return client.MaintenanceWindow{}, dir, false
	}
	w, err := m.toAPI(dir)
	if err != nil {
		diags.AddError("Invalid maintenance window", err.Error())
		return client.MaintenanceWindow{}, dir, false
	}
	return w, dir, true
}

func (r *maintenanceWindowResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan maintenanceWindowModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	w, dir, ok := r.build(ctx, plan, &resp.Diagnostics)
	if !ok {
		return
	}
	created, err := r.client.CreateMaintenanceWindow(ctx, w)
	if err != nil {
		resp.Diagnostics.AddError("Creating maintenance window", err.Error())
		return
	}
	plan.fromAPI(created, dir)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *maintenanceWindowResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state maintenanceWindowModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetMaintenanceWindow(ctx, state.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Reading maintenance window", err.Error())
		return
	}
	dir, err := r.rules(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Listing alert rules", err.Error())
		return
	}
	state.fromAPI(got, dir)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *maintenanceWindowResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state maintenanceWindowModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	w, dir, ok := r.build(ctx, plan, &resp.Diagnostics)
	if !ok {
		return
	}
	updated, err := r.client.UpdateMaintenanceWindow(ctx, state.ID.ValueString(), w)
	if err != nil {
		resp.Diagnostics.AddError("Updating maintenance window", err.Error())
		return
	}
	plan.fromAPI(updated, dir)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *maintenanceWindowResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state maintenanceWindowModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteMaintenanceWindow(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Deleting maintenance window", err.Error())
	}
}

// ImportState accepts a window id or its name.
func (r *maintenanceWindowResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := req.ID
	if !guidPattern.MatchString(id) {
		found, err := r.client.FindMaintenanceWindowByName(ctx, id)
		if err != nil {
			resp.Diagnostics.AddError("Importing maintenance window", err.Error())
			return
		}
		id = found.ID
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}
