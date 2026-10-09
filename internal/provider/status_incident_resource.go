package provider

import (
	"context"
	"strings"

	"github.com/aminparsa18/terraform-provider-flare/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*statusIncidentResource)(nil)
	_ resource.ResourceWithConfigure   = (*statusIncidentResource)(nil)
	_ resource.ResourceWithImportState = (*statusIncidentResource)(nil)
)

var statusIncidentStatuses = []string{"Investigating", "Identified", "Monitoring", "Resolved"}

// NewStatusIncidentResource is the flare_status_incident factory.
func NewStatusIncidentResource() resource.Resource { return &statusIncidentResource{} }

type statusIncidentResource struct{ client *client.Client }

type statusIncidentModel struct {
	ID         types.String `tfsdk:"id"`
	PageID     types.String `tfsdk:"page_id"`
	Title      types.String `tfsdk:"title"`
	Status     types.String `tfsdk:"status"`
	Message    types.String `tfsdk:"message"`
	Components types.Set    `tfsdk:"components"`
}

func (r *statusIncidentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_status_incident"
}

func (r *statusIncidentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A written incident on a status page (ADR-0159). `status` and `message` are the incident's latest update: " +
			"creating the resource opens the incident with that first update, and changing `status`, `message` or `components` " +
			"posts a new update, so the incident's timeline keeps its history. Destroying it deletes the incident from the public page.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true, Description: "Flare's id for the incident.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"page_id": schema.StringAttribute{
				Required: true, Description: "Id of the `flare_status_page` the incident belongs to. Changing it replaces the incident.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"title": schema.StringAttribute{
				Required: true, Description: "What is happening, at most 200 characters. Changing it replaces the incident, since the API cannot rename one.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"status": schema.StringAttribute{
				Required: true, Description: "Latest update's status, one of: " + joinQuoted(statusIncidentStatuses) + ". `Resolved` closes the incident.",
				Validators: []validator.String{stringvalidator.OneOf(statusIncidentStatuses...)},
			},
			"message": schema.StringAttribute{
				Required: true, Description: "Latest update's text, at most 4000 characters. It is public.",
			},
			"components": schema.SetAttribute{
				ElementType: types.StringType, Optional: true, Computed: true,
				Default:     setdefault.StaticValue(types.SetValueMust(types.StringType, []attr.Value{})),
				Description: "`ref_id`s (monitor or SLO ids) of the page components the incident affects; each must be on the page.",
			},
		},
	}
}

func (r *statusIncidentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (m *statusIncidentModel) fromAPI(api client.StatusIncident) {
	m.ID = types.StringValue(api.ID)
	m.PageID = types.StringValue(api.PageID)
	m.Title = types.StringValue(api.Title)
	m.Status = types.StringValue(api.Status)
	m.Message = types.StringValue("")
	if n := len(api.Updates); n > 0 {
		m.Message = types.StringValue(api.Updates[n-1].Message)
	}
	m.Components = stringSet(lowerAll(api.Components))
}

func lowerAll(values []string) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = strings.ToLower(v)
	}
	return out
}

func (r *statusIncidentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan statusIncidentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	components := []string{}
	resp.Diagnostics.Append(setToStrings(ctx, plan.Components, &components)...)
	if resp.Diagnostics.HasError() {
		return
	}
	created, err := r.client.OpenStatusIncident(ctx, plan.PageID.ValueString(), client.StatusIncidentRequest{
		Title: plan.Title.ValueString(), Status: plan.Status.ValueString(), Message: plan.Message.ValueString(), Components: components,
	})
	if err != nil {
		resp.Diagnostics.AddError("Opening status incident", err.Error())
		return
	}
	plan.fromAPI(created)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *statusIncidentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state statusIncidentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetStatusIncident(ctx, state.PageID.ValueString(), state.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Reading status incident", err.Error())
		return
	}
	state.fromAPI(got)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *statusIncidentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state statusIncidentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	components := []string{}
	resp.Diagnostics.Append(setToStrings(ctx, plan.Components, &components)...)
	if resp.Diagnostics.HasError() {
		return
	}
	updated, err := r.client.PostStatusIncidentUpdate(ctx, state.PageID.ValueString(), state.ID.ValueString(), client.StatusIncidentUpdateRequest{
		Status: plan.Status.ValueString(), Message: plan.Message.ValueString(), Components: components,
	})
	if err != nil {
		resp.Diagnostics.AddError("Updating status incident", err.Error())
		return
	}
	plan.fromAPI(updated)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *statusIncidentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state statusIncidentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteStatusIncident(ctx, state.PageID.ValueString(), state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Deleting status incident", err.Error())
	}
}

// ImportState accepts `<page id>/<incident id>`.
func (r *statusIncidentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	page, id, ok := strings.Cut(req.ID, "/")
	if !ok || !guidPattern.MatchString(page) || !guidPattern.MatchString(id) {
		resp.Diagnostics.AddError("Importing status incident", "expected `<page id>/<incident id>`, both GUIDs.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, pathRoot("page_id"), page)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, pathRoot("id"), id)...)
}
