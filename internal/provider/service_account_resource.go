package provider

import (
	"context"

	"github.com/aminparsa18/terraform-provider-flare/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*serviceAccountResource)(nil)
	_ resource.ResourceWithConfigure   = (*serviceAccountResource)(nil)
	_ resource.ResourceWithImportState = (*serviceAccountResource)(nil)
)

var userRoles = []string{"Admin", "Member", "Viewer"}

// NewServiceAccountResource is the flare_service_account factory.
func NewServiceAccountResource() resource.Resource { return &serviceAccountResource{} }

type serviceAccountResource struct{ client *client.Client }

type serviceAccountModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
	Role types.String `tfsdk:"role"`
}

func (r *serviceAccountResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_account"
}

func (r *serviceAccountResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A service account: a non-human identity that authenticates with access tokens. Flare has no API to rename it " +
			"or change its role, so changing either replaces the account (and invalidates its tokens). Destroying it deletes it permanently.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true, Description: "Flare's id for the account.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required: true, Description: "Unique (case-insensitive) name, at most 64 characters. Changing it replaces the account.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"role": schema.StringAttribute{
				Required: true, Description: "One of: " + joinQuoted(userRoles) + ". Changing it replaces the account.",
				Validators:    []validator.String{stringvalidator.OneOf(userRoles...)},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *serviceAccountResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *serviceAccountResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serviceAccountModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	created, err := r.client.CreateServiceAccount(ctx, plan.Name.ValueString(), plan.Role.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Creating service account", err.Error())
		return
	}
	plan.ID = types.StringValue(created.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *serviceAccountResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serviceAccountModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := r.client.GetServiceAccount(ctx, state.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Reading service account", err.Error())
		return
	}
	state.Name = types.StringValue(got.Username)
	state.Role = types.StringValue(got.Role)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Update is never reached with a change (name and role force replacement); it only keeps the framework happy.
func (r *serviceAccountResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serviceAccountModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *serviceAccountResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serviceAccountModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteServiceAccount(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Deleting service account", err.Error())
	}
}

// ImportState accepts an account id or its name.
func (r *serviceAccountResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := req.ID
	if !guidPattern.MatchString(id) {
		found, err := r.client.FindServiceAccountByName(ctx, id)
		if err != nil {
			resp.Diagnostics.AddError("Importing service account", err.Error())
			return
		}
		id = found.ID
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}
