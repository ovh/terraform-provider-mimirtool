package provider

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func hash(s string) string {
	sha := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sha[:])
}

// mapStringFromTypesMap converts a types.Map to map[string]string for template handling.
func mapStringFromTypesMap(m types.Map) map[string]string {
	if m.IsNull() || m.IsUnknown() {
		return nil
	}
	result := make(map[string]string, len(m.Elements()))
	for k, v := range m.Elements() {
		if strVal, ok := v.(types.String); ok && !strVal.IsNull() && !strVal.IsUnknown() {
			result[k] = strVal.ValueString()
		}
	}
	return result
}

// typeMapFromMapString converts a map[string]string (e.g., Alertmanager templates)
// into a Terraform types.Map value, where each value is a types.StringValue.
// This is useful for storing string maps in Terraform state.
func typeMapFromMapString(templates map[string]string) types.Map {
	if templates == nil {
		return types.MapNull(types.StringType)
	}

	templatesMap := make(map[string]attr.Value, len(templates))
	for k, v := range templates {
		templatesMap[k] = types.StringValue(v)
	}

	return types.MapValueMust(types.StringType, templatesMap)
}
