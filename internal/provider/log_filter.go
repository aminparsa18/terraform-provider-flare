package provider

import (
	"github.com/aminparsa18/terraform-provider-flare/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// logFilterAttribute is the log-filter block shared by alert rules (log_condition) and pipeline rules
// (condition). Both map it onto the API's LogFilter.
func logFilterAttribute(desc string) schema.SingleNestedAttribute {
	strList := func(d string) schema.ListAttribute {
		return schema.ListAttribute{Optional: true, ElementType: types.StringType, Description: d}
	}
	return schema.SingleNestedAttribute{
		Optional:    true,
		Description: desc,
		Attributes: map[string]schema.Attribute{
			"services":         strList("Only logs from these services."),
			"severity_numbers": schema.ListAttribute{Optional: true, ElementType: types.Int64Type, Description: "Only these OTel severity numbers (1-24)."},
			"search":           schema.StringAttribute{Optional: true, Description: "Substring match on the log body."},
			"scope_names":      strList("Only logs from these instrumentation scopes (a trailing `*` matches a prefix)."),
			"attributes": schema.ListNestedAttribute{
				Optional:    true,
				Description: "Attribute equality filters.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"bag": schema.StringAttribute{
						Optional: true, Computed: true, Default: stringdefault.StaticString("Log"),
						Description: "Which attribute set the key belongs to. One of: " + joinQuoted(attributeBags) + ". Defaults to `Log`.",
						Validators:  []validator.String{stringvalidator.OneOf(attributeBags...)},
					},
					"key":   schema.StringAttribute{Required: true},
					"value": schema.StringAttribute{Required: true},
				}},
			},
		},
	}
}

func (c *logConditionModel) toAPI() *client.LogFilter {
	if c == nil {
		return nil
	}
	f := &client.LogFilter{Services: stringSlice(c.Services), ScopeNames: stringSlice(c.ScopeNames), Search: strPtr(c.Search)}
	for _, n := range c.SeverityNumbers {
		f.SeverityNumbers = append(f.SeverityNumbers, int(n.ValueInt64()))
	}
	for _, a := range c.Attributes {
		f.Attributes = append(f.Attributes, client.AttributeFilter{Bag: a.Bag.ValueString(), Key: a.Key.ValueString(), Value: a.Value.ValueString()})
	}
	return f
}
