package provider

import (
	"context"

	"github.com/aminparsa18/terraform-provider-flare/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
	_ resource.Resource                = (*metricAttributeRuleResource)(nil)
	_ resource.ResourceWithConfigure   = (*metricAttributeRuleResource)(nil)
	_ resource.ResourceWithImportState = (*metricAttributeRuleResource)(nil)
)

var metricAttributeRuleModes = []string{"Drop", "KeepOnly"}

// NewMetricAttributeRuleResource is the flare_metric_attribute_rule factory.
func NewMetricAttributeRuleResource() resource.Resource { return &metricAttributeRuleResource{} }

type metricAttributeRuleResource struct{ client *client.Client }

type metricAttributeRuleModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Enabled     types.Bool   `tfsdk:"enabled"`
	MetricName  types.String `tfsdk:"metric_name"`
	Mode        types.String `tfsdk:"mode"`
	Attributes  types.Set    `tfsdk:"attributes"`
}

func (r *metricAttributeRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_metric_attribute_rule"
}

func (r *metricAttributeRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A metric attribute-reduction rule: strips high-cardinality data-point attributes from a metric at ingest, " +
			"before they multiply its series count.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true, Description: "Flare's id for the rule.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{Required: true, Description: "Unique (case-insensitive) name. Renaming updates the rule in place."},
			"description": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: "Free-text description.",
			},
			"enabled": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(true), Description: "Whether the rule is applied. Defaults to true.",
			},
			"metric_name": schema.StringAttribute{
				Required: true, Description: "Exact metric name, or a prefix followed by a single trailing `*` (`http.client.*`).",
			},
			"mode": schema.StringAttribute{
				Required: true, Description: "`Drop` removes the listed attributes and keeps the rest; `KeepOnly` keeps only the listed ones. One of: " + joinQuoted(metricAttributeRuleModes) + ".",
				Validators: []validator.String{stringvalidator.OneOf(metricAttributeRuleModes...)},
			},
			"attributes": schema.SetAttribute{
				Required: true, ElementType: types.StringType, Description: "Data-point attribute keys the mode applies to.",
				Validators: []validator.Set{setvalidator.SizeAtLeast(1)},
			},
		},
	}
}

func (r *metricAttributeRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (m metricAttributeRuleModel) toAPI(ctx context.Context) (client.MetricAttributeRule, diag.Diagnostics) {
	var attrs []string
	if d := m.Attributes.ElementsAs(ctx, &attrs, false); d.HasError() {
		return client.MetricAttributeRule{}, d
	}
	return client.MetricAttributeRule{
		Name: m.Name.ValueString(), Description: strPtr(m.Description), Enabled: boolPtr(m.Enabled),
		MetricName: m.MetricName.ValueString(), Mode: m.Mode.ValueString(), Attributes: attrs,
	}, nil
}

func (m *metricAttributeRuleModel) fromAPI(api client.MetricAttributeRule) {
	m.ID = types.StringValue(api.ID)
	m.Name = types.StringValue(api.Name)
	m.Description = types.StringValue(deref(api.Description))
	m.Enabled = types.BoolValue(api.Enabled == nil || *api.Enabled)
	m.MetricName = types.StringValue(api.MetricName)
	m.Mode = types.StringValue(api.Mode)
	elems := make([]string, 0, len(api.Attributes))
	elems = append(elems, api.Attributes...)
	set, _ := types.SetValueFrom(context.Background(), types.StringType, elems)
	m.Attributes = set
}

func (r *metricAttributeRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan metricAttributeRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body, diags := plan.toAPI(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	created, err := r.client.CreateMetricAttributeRule(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Creating metric attribute rule", err.Error())
		return
	}
	plan.fromAPI(created)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *metricAttributeRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state metricAttributeRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetMetricAttributeRule(ctx, state.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Reading metric attribute rule", err.Error())
		return
	}
	state.fromAPI(got)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *metricAttributeRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state metricAttributeRuleModel
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
	updated, err := r.client.UpdateMetricAttributeRule(ctx, state.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Updating metric attribute rule", err.Error())
		return
	}
	plan.fromAPI(updated)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *metricAttributeRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state metricAttributeRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteMetricAttributeRule(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Deleting metric attribute rule", err.Error())
	}
}

// ImportState accepts a rule id or its name.
func (r *metricAttributeRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := req.ID
	if !guidPattern.MatchString(id) {
		found, err := r.client.FindMetricAttributeRuleByName(ctx, id)
		if err != nil {
			resp.Diagnostics.AddError("Importing metric attribute rule", err.Error())
			return
		}
		id = found.ID
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}
