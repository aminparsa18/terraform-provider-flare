package provider

import (
	"strings"

	"github.com/aminparsa18/terraform-provider-flare/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func pathRoot(name string) path.Path { return path.Root(name) }

// configuredClient unwraps the provider's client from ProviderData, which is nil until Configure ran.
func configuredClient(data any, diags *diag.Diagnostics) *client.Client {
	if data == nil {
		return nil
	}
	c, ok := data.(*client.Client)
	if !ok {
		diags.AddError("Unexpected provider data", "Expected *client.Client; this is a bug in the provider.")
		return nil
	}
	return c
}

// strPtr sends nothing for null/unknown, and the value (including "") otherwise.
func strPtr(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	return &s
}

// readString maps an API string onto state: the API returns "" for an unset field, and a configuration
// that left the attribute null must keep reading back as null or Terraform reports a spurious diff.
func readString(api *string, prior types.String) types.String {
	if api == nil || *api == "" {
		if prior.IsNull() || prior.IsUnknown() {
			return types.StringNull()
		}
		return types.StringValue("")
	}
	return types.StringValue(*api)
}

func boolPtr(v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	b := v.ValueBool()
	return &b
}

func intPtr(v types.Int64) *int {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	i := int(v.ValueInt64())
	return &i
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt(i *int, def int) int {
	if i == nil {
		return def
	}
	return *i
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func lower(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
