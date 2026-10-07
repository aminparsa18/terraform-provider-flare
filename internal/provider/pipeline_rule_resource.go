package provider

import (
	"context"

	"github.com/aminparsa18/terraform-provider-flare/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
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
	_ resource.Resource                = (*pipelineRuleResource)(nil)
	_ resource.ResourceWithConfigure   = (*pipelineRuleResource)(nil)
	_ resource.ResourceWithImportState = (*pipelineRuleResource)(nil)
)

var pipelineActionKinds = []string{"ExtractRegex", "RedactRegex", "ParseJson"}

// NewPipelineRuleResource is the flare_pipeline_rule factory.
func NewPipelineRuleResource() resource.Resource { return &pipelineRuleResource{} }

type pipelineRuleResource struct{ client *client.Client }

type pipelineRuleModel struct {
	ID          types.String          `tfsdk:"id"`
	Name        types.String          `tfsdk:"name"`
	Description types.String          `tfsdk:"description"`
	Enabled     types.Bool            `tfsdk:"enabled"`
	Condition   *logConditionModel    `tfsdk:"condition"`
	Actions     []pipelineActionModel `tfsdk:"actions"`
}

// pipelineActionModel is the flat shape of one action: kind selects which of the other attributes apply.
type pipelineActionModel struct {
	Kind               types.String `tfsdk:"kind"`
	SourceAttributeKey types.String `tfsdk:"source_attribute_key"`
	Pattern            types.String `tfsdk:"pattern"`
	Replacement        types.String `tfsdk:"replacement"`
	KeyPrefix          types.String `tfsdk:"key_prefix"`
	MaxDepth           types.Int64  `tfsdk:"max_depth"`
	MaxKeys            types.Int64  `tfsdk:"max_keys"`
}

// redactDefaultReplacement is what Flare uses when a RedactRegex action sends no replacement.
const redactDefaultReplacement = "***"

func (r *pipelineRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_pipeline_rule"
}

func (r *pipelineRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "An ingest-time pipeline rule: redacts, extracts or parses log fields as events are flushed. " +
			"Actions run in list order against every log matching `condition`. An omitted `condition` matches every log.",
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
			"condition": logFilterAttribute("Which logs the rule applies to. Omit to match every log."),
			"actions": schema.ListNestedAttribute{
				Required:    true,
				Description: "Ordered actions; each sees the previous action's output.",
				Validators:  []validator.List{listvalidator.SizeAtLeast(1)},
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"kind": schema.StringAttribute{
						Required: true, Description: "One of: " + joinQuoted(pipelineActionKinds) + ".",
						Validators: []validator.String{stringvalidator.OneOf(pipelineActionKinds...)},
					},
					"source_attribute_key": schema.StringAttribute{Optional: true, Description: "Log attribute to read; omit to use the log body."},
					"pattern":              schema.StringAttribute{Optional: true, Description: "Regex. Required for `ExtractRegex` (needs at least one named group) and `RedactRegex`."},
					"replacement": schema.StringAttribute{
						Optional: true, Description: "`RedactRegex` only: text that replaces every match. Flare defaults to `" + redactDefaultReplacement + "`.",
					},
					"key_prefix": schema.StringAttribute{Optional: true, Description: "`ParseJson` only: prepended to every flattened key."},
					"max_depth": schema.Int64Attribute{
						Optional: true, Description: "`ParseJson` only: object levels to flatten (1-10). Flare defaults to 5.",
						Validators: []validator.Int64{int64validator.Between(1, 10)},
					},
					"max_keys": schema.Int64Attribute{
						Optional: true, Description: "`ParseJson` only: stop after this many attributes (1-500). Flare defaults to 100.",
						Validators: []validator.Int64{int64validator.Between(1, 500)},
					},
				}},
			},
		},
	}
}

func (r *pipelineRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (a pipelineActionModel) toAPI() client.PipelineAction {
	out := client.PipelineAction{Kind: a.Kind.ValueString()}
	switch out.Kind {
	case "ExtractRegex":
		out.ExtractRegex = &client.ExtractRegex{SourceAttributeKey: strPtr(a.SourceAttributeKey), Pattern: a.Pattern.ValueString()}
	case "RedactRegex":
		out.RedactRegex = &client.RedactRegex{SourceAttributeKey: strPtr(a.SourceAttributeKey), Pattern: a.Pattern.ValueString(), Replacement: strPtr(a.Replacement)}
	case "ParseJson":
		out.ParseJSON = &client.ParseJSON{
			SourceAttributeKey: strPtr(a.SourceAttributeKey), KeyPrefix: strPtr(a.KeyPrefix),
			MaxDepth: intPtr(a.MaxDepth), MaxKeys: intPtr(a.MaxKeys),
		}
	}
	return out
}

// fromAPI maps one action back, using the configured action at the same index as `prior` so an attribute the
// configuration left null (or empty) reads back unchanged instead of producing a spurious diff.
func pipelineActionFromAPI(api client.PipelineAction, prior pipelineActionModel) pipelineActionModel {
	out := pipelineActionModel{
		Kind: types.StringValue(api.Kind), SourceAttributeKey: types.StringNull(), Pattern: types.StringNull(),
		Replacement: types.StringNull(), KeyPrefix: types.StringNull(), MaxDepth: types.Int64Null(), MaxKeys: types.Int64Null(),
	}
	switch {
	case api.ExtractRegex != nil:
		out.SourceAttributeKey = readString(api.ExtractRegex.SourceAttributeKey, prior.SourceAttributeKey)
		out.Pattern = types.StringValue(api.ExtractRegex.Pattern)
	case api.RedactRegex != nil:
		out.SourceAttributeKey = readString(api.RedactRegex.SourceAttributeKey, prior.SourceAttributeKey)
		out.Pattern = types.StringValue(api.RedactRegex.Pattern)
		// An omitted replacement comes back as Flare's default; keep it null if that is what was configured.
		if api.RedactRegex.Replacement != nil && !(prior.Replacement.IsNull() && *api.RedactRegex.Replacement == redactDefaultReplacement) {
			out.Replacement = types.StringValue(*api.RedactRegex.Replacement)
		}
	case api.ParseJSON != nil:
		out.SourceAttributeKey = readString(api.ParseJSON.SourceAttributeKey, prior.SourceAttributeKey)
		out.KeyPrefix = readString(api.ParseJSON.KeyPrefix, prior.KeyPrefix)
		if api.ParseJSON.MaxDepth != nil {
			out.MaxDepth = types.Int64Value(int64(*api.ParseJSON.MaxDepth))
		}
		if api.ParseJSON.MaxKeys != nil {
			out.MaxKeys = types.Int64Value(int64(*api.ParseJSON.MaxKeys))
		}
	}
	return out
}

func (m pipelineRuleModel) toAPI() client.PipelineRule {
	out := client.PipelineRule{
		Name: m.Name.ValueString(), Description: strPtr(m.Description), Enabled: boolPtr(m.Enabled),
		Condition: m.Condition.toAPI(), Actions: []client.PipelineAction{},
	}
	for _, a := range m.Actions {
		out.Actions = append(out.Actions, a.toAPI())
	}
	return out
}

func (m *pipelineRuleModel) fromAPI(api client.PipelineRule) {
	m.ID = types.StringValue(api.ID)
	m.Name = types.StringValue(api.Name)
	m.Description = types.StringValue(deref(api.Description))
	m.Enabled = types.BoolValue(api.Enabled == nil || *api.Enabled)
	m.Condition = logConditionFromAPI(api.Condition, m.Condition)
	prior := m.Actions
	m.Actions = make([]pipelineActionModel, len(api.Actions))
	for i, a := range api.Actions {
		var p pipelineActionModel
		if i < len(prior) {
			p = prior[i]
		}
		m.Actions[i] = pipelineActionFromAPI(a, p)
	}
}

func (r *pipelineRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan pipelineRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	created, err := r.client.CreatePipelineRule(ctx, plan.toAPI())
	if err != nil {
		resp.Diagnostics.AddError("Creating pipeline rule", err.Error())
		return
	}
	plan.fromAPI(created)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *pipelineRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state pipelineRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetPipelineRule(ctx, state.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Reading pipeline rule", err.Error())
		return
	}
	state.fromAPI(got)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *pipelineRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state pipelineRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	updated, err := r.client.UpdatePipelineRule(ctx, state.ID.ValueString(), plan.toAPI())
	if err != nil {
		resp.Diagnostics.AddError("Updating pipeline rule", err.Error())
		return
	}
	plan.fromAPI(updated)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *pipelineRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state pipelineRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeletePipelineRule(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Deleting pipeline rule", err.Error())
	}
}

// ImportState accepts a rule id or its name.
func (r *pipelineRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := req.ID
	if !guidPattern.MatchString(id) {
		found, err := r.client.FindPipelineRuleByName(ctx, id)
		if err != nil {
			resp.Diagnostics.AddError("Importing pipeline rule", err.Error())
			return
		}
		id = found.ID
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}
