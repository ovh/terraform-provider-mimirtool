package mimirtool

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/grafana/mimir/pkg/mimirtool/rules"
	"github.com/grafana/mimir/pkg/mimirtool/rules/rwrulefmt"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
)

func resourceRulerNamespace() *schema.Resource {
	return &schema.Resource{
		Description: `
* [Official documentation](https://grafana.com/docs/mimir/latest/)
* [HTTP API](https://grafana.com/docs/mimir/latest/operators-guide/reference-http-api/#ruler)
`,

		CreateContext: rulerNamespaceCreate,
		ReadContext:   rulerNamespaceRead,
		UpdateContext: rulerNamespaceUpdate,
		DeleteContext: rulerNamespaceDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"namespace": {
				Description: "The name of the namespace to create in Grafana Mimir.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"config_yaml": {
				Description:      "The namespace's groups rules definition to create in Grafana Mimir as YAML.",
				Type:             schema.TypeString,
				StateFunc:        normalizeNamespaceYAML,
				ValidateDiagFunc: validateNamespaceYAML,
				DiffSuppressFunc: diffNamespaceYAML,
				Required:         true,
			},
			"strict_recording_rule_check": {
				Description: "Fails rules checks that do not match best practices exactly. See: https://prometheus.io/docs/practices/rules/",
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
			},
		},
	}
}

func getRuleNamespaceFromYAML(ctx context.Context, configYAML string) (rules.RuleNamespace, error) {
	var ruleNamespace rules.RuleNamespace
	// We pass only one ruleGroup while ParseBytes return an array, we only need the first element
	ruleNamespaces, err := rules.ParseBytes([]byte(configYAML))
	if err != nil {
		return ruleNamespace, fmt.Errorf("failed to parse namespace definition:\n%s", err)
	}

	if len(ruleNamespaces) > 1 {
		return ruleNamespace, fmt.Errorf("namespace definition contains more than one namespace which is not supported")
	}
	if len(ruleNamespaces) == 1 {
		return ruleNamespaces[0], nil
	}
	return ruleNamespace, fmt.Errorf("no namespace definition found")
}

func checkRecordingRules(ruleNamespace rules.RuleNamespace, strict bool) error {
	invalidRulesCount := ruleNamespace.CheckRecordingRules(strict)
	if invalidRulesCount > 0 {
		return fmt.Errorf("namespace contains %d rules that don't match the requirements", invalidRulesCount)
	}
	return nil
}

func getRuleNamespacesFromMimir(ctx context.Context, d *schema.ResourceData, meta any) ([]rwrulefmt.RuleGroup, error) {
	client := meta.(*client).cli
	namespace := d.Get("namespace").(string)
	// namespace is required as per the definition above as such we have either nothing or one namespace

	rulesGroups, err := client.ListRules(ctx, namespace)
	if err != nil {
		return nil, err
	}
	return rulesGroups[namespace], nil
}

func rulerNamespaceCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	client := meta.(*client).cli
	namespace := d.Get("namespace").(string)
	ruleGroup := d.Get("config_yaml").(string)
	strictRecordingRuleCheck := d.Get("strict_recording_rule_check").(bool)

	ruleNamespace, err := getRuleNamespaceFromYAML(ctx, ruleGroup)
	if err != nil {
		return diag.FromErr(err)
	}

	err = checkRecordingRules(ruleNamespace, strictRecordingRuleCheck)
	if err != nil {
		return diag.FromErr(err)
	}

	for _, group := range ruleNamespace.Groups {
		err = client.CreateRuleGroup(ctx, namespace, group)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	d.SetId(hash(namespace))
	return rulerNamespaceRead(ctx, d, meta)
}

func rulerNamespaceRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	client := meta.(*client).cli
	namespace := d.Get("namespace").(string)

	remoteNamespaceRuleGroup, err := client.ListRules(ctx, namespace)
	if err != nil {
		return diag.FromErr(err)
	}
	// Mimir top level key is the namespace name while in the YAML definition the top level key is groups
	// Let's rename the key to be able to have a nice difference
	remoteNamespaceRuleGroup["groups"] = remoteNamespaceRuleGroup[namespace]
	delete(remoteNamespaceRuleGroup, namespace)

	configYAML, err := yaml.Marshal(remoteNamespaceRuleGroup)
	if err != nil {
		return diag.FromErr(err)
	}
	d.Set("config_yaml", normalizeNamespaceYAML(string(configYAML)))
	return diags
}

func rulerNamespaceUpdate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	client := meta.(*client).cli
	namespace := d.Get("namespace").(string)
	ruleGroup := d.Get("config_yaml").(string)
	strictRecordingRuleCheck := d.Get("strict_recording_rule_check").(bool)

	errDiag := rulerNamespaceCreate(ctx, d, meta)
	if errDiag != nil {
		return errDiag
	}
	// Clean up the rules which need to be updated have been so with rulerNamespaceCreate,
	// we still need to delete the rules which have been removed from the definition.
	ruleNamespace, err := getRuleNamespaceFromYAML(ctx, ruleGroup)
	if err != nil {
		return diag.FromErr(err)
	}
	err = checkRecordingRules(ruleNamespace, strictRecordingRuleCheck)
	if err != nil {
		return diag.FromErr(err)
	}

	var nsGroupNames []string
	for _, group := range ruleNamespace.Groups {
		nsGroupNames = append(nsGroupNames, group.Name)
	}

	// the ones which are configured in the rulers as per rulerNamespaceRead
	localNamespaces, err := getRuleNamespacesFromMimir(ctx, d, meta)
	if err != nil {
		return diag.FromErr(err)
	}
	currentGroupsNames := make([]string, 0, len(localNamespaces))
	for _, group := range localNamespaces {
		currentGroupsNames = append(currentGroupsNames, group.Name)
	}

	// All groups present in Mimir but not in the YAML definition must be deleted
	for _, name := range currentGroupsNames {
		if !slices.Contains(nsGroupNames, name) {
			errRaw := client.DeleteRuleGroup(ctx, namespace, name)
			if err != nil {
				return diag.FromErr(errRaw)
			}
		}
	}
	return diags
}

func rulerNamespaceDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	client := meta.(*client).cli
	namespace := d.Get("namespace").(string)

	err := client.DeleteNamespace(ctx, namespace)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return diags
}

// Borrowed from https://github.com/grafana/terraform-provider-grafana/blob/master/grafana/resource_dashboard.go
func normalizeNamespaceYAML(config any) string {
	configYAML := config.(string)
	var ruleNamespace rules.RuleNamespace

	yaml.Unmarshal([]byte(configYAML), &ruleNamespace)
	ruleNamespace.LintExpressions(rules.MimirBackend)

	namespaceBytes, _ := yaml.Marshal(ruleNamespace.Groups)
	if storeRulesSHA256 {
		configHash := sha256.Sum256(namespaceBytes)
		return fmt.Sprintf("%x", configHash[:])
	}
	return string(namespaceBytes)
}

func validateNamespaceYAML(config any, k cty.Path) diag.Diagnostics {
	var diags diag.Diagnostics
	configYAML := config.(string)
	_, err := getRuleNamespaceFromYAML(context.Background(), configYAML)
	if err != nil {
		return diag.Diagnostics{
			diag.Diagnostic{
				Severity:      diag.Error,
				Summary:       "Namespace definition is not valid.",
				Detail:        err.Error(),
				AttributePath: k,
			},
		}
	}
	return diags
}

func diffNamespaceYAML(_, oldValue, newValue string, _ *schema.ResourceData) bool {
	var (
		oldConfig []rwrulefmt.RuleGroup
		newConfig []rwrulefmt.RuleGroup
		err       error
	)

	// If we cannot unmarshal, as we cannot return an error, let's say there is a difference
	err = yaml.Unmarshal([]byte(newValue), &newConfig)
	if err != nil {
		tflog.Warn(context.Background(), "Failed to unmarshal newConfigYaml")
		tflog.Debug(context.Background(), err.Error())
		return false
	}
	err = yaml.Unmarshal([]byte(oldValue), &oldConfig)
	if err != nil {
		tflog.Warn(context.Background(), "Failed to unmarshal oldValue")
		tflog.Debug(context.Background(), err.Error())
		return false
	}

	return rules.CompareNamespaces(
		rules.RuleNamespace{
			Namespace: "old",
			Filepath:  "",
			Groups:    oldConfig,
		},
		rules.RuleNamespace{
			Namespace: "new",
			Filepath:  "",
			Groups:    newConfig,
		}).State == rules.Unchanged
}
