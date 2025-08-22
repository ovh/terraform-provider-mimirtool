package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"gopkg.in/yaml.v3"
)

// Validator for valid namespace YAML
// Ensures the YAML is a valid single namespace definition
// Returns a diagnostic error if not

type namespaceYAMLValidator struct{}

func (v namespaceYAMLValidator) Description(_ context.Context) string {
	return "Validates that the YAML is a valid single namespace definition"
}

func (v namespaceYAMLValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v namespaceYAMLValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.ValueString() == "" {
		// Let the non-empty validator handle this case
		return
	}
	_, err := getRuleNamespaceFromYAML(ctx, req.ConfigValue.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid namespace YAML",
			fmt.Sprintf("Namespace definition is not valid: %s", err.Error()),
		)
	}
}

// yamlSyntaxValidator checks that a string is valid YAML

type yamlSyntaxValidator struct{}

func (v yamlSyntaxValidator) Description(_ context.Context) string {
	return "Ensures the string is valid YAML syntax"
}

func (v yamlSyntaxValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v yamlSyntaxValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.ValueString() == "" {
		return
	}
	var temp interface{}
	err := yaml.Unmarshal([]byte(req.ConfigValue.ValueString()), &temp)
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid YAML syntax",
			err.Error(),
		)
	}
}
