package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/aminparsa18/terraform-provider-flare/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
	_ resource.Resource                = (*dashboardResource)(nil)
	_ resource.ResourceWithConfigure   = (*dashboardResource)(nil)
	_ resource.ResourceWithImportState = (*dashboardResource)(nil)
)

// NewDashboardResource is the flare_dashboard factory.
func NewDashboardResource() resource.Resource { return &dashboardResource{} }

type dashboardResource struct{ client *client.Client }

type dashboardModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Tags        types.Set    `tfsdk:"tags"`
	ProjectID   types.String `tfsdk:"project_id"`
	LayoutJSON  types.String `tfsdk:"layout_json"`
}

// layoutObject checks the one thing the provider can know about a layout: it is a JSON object with a `panels`
// array. Everything else (panel types, queries, variables) is validated by Flare when the dashboard is saved.
type layoutObject struct{}

func (layoutObject) Description(context.Context) string {
	return "must be a JSON object with a `panels` array"
}
func (v layoutObject) MarkdownDescription(ctx context.Context) string { return v.Description(ctx) }
func (layoutObject) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if err := checkLayout(req.ConfigValue.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid dashboard layout", err.Error())
	}
}

func checkLayout(s string) error {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s), &obj); err != nil {
		return fmt.Errorf("layout_json must be a JSON object: %w", err)
	}
	panels, ok := obj["panels"]
	if !ok {
		return fmt.Errorf("layout_json must have a `panels` array")
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(panels, &arr); err != nil {
		return fmt.Errorf("layout_json `panels` must be an array")
	}
	return nil
}

type lowercaseTag struct{}

func (lowercaseTag) Description(context.Context) string {
	return "must be lowercase with no surrounding spaces, at most 32 characters"
}
func (v lowercaseTag) MarkdownDescription(ctx context.Context) string { return v.Description(ctx) }
func (lowercaseTag) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	t := req.ConfigValue.ValueString()
	if t == "" || t != lower(t) || len(t) > 32 {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid tag", fmt.Sprintf("tag %q must be non-empty, lowercase, trimmed and at most 32 characters (Flare lowercases tags)", t))
	}
}

func (r *dashboardResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dashboard"
}

func (r *dashboardResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A dashboard. The layout (panels, rows, variables) is Flare's own JSON and is passed through as-is: author it in the " +
			"dashboard editor and export it, then keep the file with `file()`, or write it with `jsonencode()`. The provider checks only that it is " +
			"a JSON object with a `panels` array; Flare validates the rest. The token must belong to an Admin or Member; a dashboard owned by " +
			"someone else can only be changed by an Admin (or a project admin), otherwise Flare answers 403.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true, Description: "Flare's id for the dashboard.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{Required: true, Description: "Unique (case-insensitive) within its project; instance-wide dashboards share one namespace. Renaming updates in place."},
			"description": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Free-text description.",
			},
			"tags": schema.SetAttribute{
				Optional: true, ElementType: types.StringType,
				Description: "Up to 10 lowercase tags the Dashboards page filters by.",
				Validators: []validator.Set{
					setvalidator.SizeAtMost(10),
					setvalidator.ValueStringsAre(lowercaseTag{}),
				},
			},
			"project_id": schema.StringAttribute{Optional: true, Description: "Owning project id; omit for an instance-wide dashboard."},
			"layout_json": schema.StringAttribute{
				Required:    true,
				Description: "The layout as a JSON object: `{\"panels\": [...], \"variables\": [...], \"rows\": [...]}`. Compared as JSON, so key order and whitespace never cause a diff.",
				Validators:  []validator.String{stringvalidator.LengthAtLeast(2), layoutObject{}},
			},
		},
	}
}

func (r *dashboardResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

// sameJSON reports whether two JSON documents are equal ignoring key order and whitespace.
func sameJSON(a, b string) bool {
	var x, y any
	if json.Unmarshal([]byte(a), &x) != nil || json.Unmarshal([]byte(b), &y) != nil {
		return false
	}
	return reflect.DeepEqual(x, y)
}

// layoutFromAPI keeps the configured text when the API's layout is the same JSON, so formatting never diffs.
func layoutFromAPI(api json.RawMessage, prior types.String) types.String {
	var compact bytes.Buffer
	text := string(api)
	if err := json.Compact(&compact, api); err == nil {
		text = compact.String()
	}
	if !prior.IsNull() && !prior.IsUnknown() && sameJSON(prior.ValueString(), text) {
		return prior
	}
	return types.StringValue(text)
}

func (m dashboardModel) toAPI(clearProject bool) (client.Dashboard, error) {
	out := client.Dashboard{
		Name: m.Name.ValueString(), Description: strPtr(m.Description), Tags: []string{},
		LayoutJSON: json.RawMessage(m.LayoutJSON.ValueString()),
	}
	if !m.Tags.IsNull() && !m.Tags.IsUnknown() {
		var tags []types.String
		m.Tags.ElementsAs(context.Background(), &tags, false)
		out.Tags = stringSlice(tags)
		if out.Tags == nil {
			out.Tags = []string{}
		}
	}
	switch p := strPtr(m.ProjectID); {
	case p != nil && *p != "":
		out.ProjectID = p
	case clearProject:
		z := emptyGUID // Flare keeps the existing project when none is sent; the empty id clears it
		out.ProjectID = &z
	}
	return out, nil
}

func (m *dashboardModel) fromAPI(api client.Dashboard) {
	m.ID = types.StringValue(api.ID)
	m.Name = types.StringValue(api.Name)
	m.Description = types.StringValue(deref(api.Description))
	m.LayoutJSON = layoutFromAPI(api.LayoutJSON, m.LayoutJSON)

	switch {
	case len(api.Tags) > 0:
		m.Tags, _ = types.SetValueFrom(context.Background(), types.StringType, api.Tags)
	case !m.Tags.IsNull() && !m.Tags.IsUnknown():
		m.Tags, _ = types.SetValueFrom(context.Background(), types.StringType, []string{})
	default:
		m.Tags = types.SetNull(types.StringType)
	}

	if api.ProjectID != nil && *api.ProjectID != "" && *api.ProjectID != emptyGUID {
		m.ProjectID = types.StringValue(*api.ProjectID)
	} else if m.ProjectID.IsNull() || m.ProjectID.ValueString() == "" {
		m.ProjectID = readString(nil, m.ProjectID)
	} else {
		m.ProjectID = types.StringNull()
	}
}

func (r *dashboardResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dashboardModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in, _ := plan.toAPI(false)
	created, err := r.client.CreateDashboard(ctx, in)
	if err != nil {
		resp.Diagnostics.AddError("Creating dashboard", err.Error())
		return
	}
	plan.fromAPI(created)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *dashboardResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dashboardModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetDashboard(ctx, state.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Reading dashboard", err.Error())
		return
	}
	state.fromAPI(got)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *dashboardResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state dashboardModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	hadProject := !state.ProjectID.IsNull() && state.ProjectID.ValueString() != ""
	in, _ := plan.toAPI(hadProject)
	updated, err := r.client.UpdateDashboard(ctx, state.ID.ValueString(), in)
	if err != nil {
		resp.Diagnostics.AddError("Updating dashboard", err.Error())
		return
	}
	plan.fromAPI(updated)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *dashboardResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dashboardModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteDashboard(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Deleting dashboard", err.Error())
	}
}

// ImportState accepts a dashboard id or its name; a name used in several projects must be imported by id.
func (r *dashboardResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := req.ID
	if !guidPattern.MatchString(id) {
		found, err := r.client.FindDashboardByName(ctx, id)
		if err != nil {
			resp.Diagnostics.AddError("Importing dashboard", err.Error())
			return
		}
		id = found.ID
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}
