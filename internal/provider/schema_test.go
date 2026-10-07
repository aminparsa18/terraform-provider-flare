package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// The framework only rejects an invalid schema (bad default, computed/required clash, ...) at runtime,
// inside Terraform. These catch it in `go test`.
func TestSchemasAreValid(t *testing.T) {
	ctx := context.Background()

	var pr provider.SchemaResponse
	New("test")().Schema(ctx, provider.SchemaRequest{}, &pr)
	if d := pr.Schema.ValidateImplementation(ctx); d.HasError() {
		t.Fatalf("provider schema: %v", d)
	}

	for _, f := range New("test")().Resources(ctx) {
		var resp resource.SchemaResponse
		f().Schema(ctx, resource.SchemaRequest{}, &resp)
		if d := resp.Schema.ValidateImplementation(ctx); d.HasError() {
			t.Errorf("resource schema: %v", d)
		}
	}
	for _, f := range New("test")().DataSources(ctx) {
		var resp datasource.SchemaResponse
		f().Schema(ctx, datasource.SchemaRequest{}, &resp)
		if d := resp.Schema.ValidateImplementation(ctx); d.HasError() {
			t.Errorf("data source schema: %v", d)
		}
	}
}
