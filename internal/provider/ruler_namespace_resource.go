package provider

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"reflect"

	"github.com/go-kit/log"
	"github.com/grafana/mimir/pkg/mimirtool/client"
	"github.com/grafana/mimir/pkg/mimirtool/rules"
	"github.com/grafana/mimir/pkg/mimirtool/rules/rwrulefmt"
	mimirtoolUtil "github.com/grafana/mimir/pkg/mimirtool/util"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/prometheus/common/model"
	"go.yaml.in/yaml/v3"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &RulerNamespaceResource{}
	_ resource.ResourceWithImportState = &RulerNamespaceResource{}
)

func NewRulerNamespaceResource() resource.Resource {
	return &RulerNamespaceResource{}
}

// RulerNamespaceResource defines the resource implementation.
type RulerNamespaceResource struct {
	client mimirClientInterface
}

// RulerNamespaceResourceModel describes the resource data model.
type RulerNamespaceResourceModel struct {
	ID                       types.String `tfsdk:"id"`
	Namespace                types.String `tfsdk:"namespace"`
	ConfigYAML               types.String `tfsdk:"config_yaml"`
	RemoteConfigYAML         types.String `tfsdk:"remote_config_yaml"`
	StrictRecordingRuleCheck types.Bool   `tfsdk:"strict_recording_rule_check"`
	RecordingRuleCheck       types.Bool   `tfsdk:"recording_rule_check"`
}

func (r *RulerNamespaceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ruler_namespace"
}

func (r *RulerNamespaceResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	tflog.Debug(ctx, "SCHEMA - init")
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Manages the rule groups of a Grafana Mimir ruler namespace. Each apply removes the groups of the namespace that are not in `config_yaml`. A configured group deleted or modified in Mimir after an apply is detected on refresh and pushed again by the next apply. [Official documentation](https://grafana.com/docs/mimir/latest/references/http-api/#ruler)",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				Description:         "hash",
				MarkdownDescription: "hash",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"namespace": schema.StringAttribute{
				MarkdownDescription: "The name of the namespace to create in Grafana Mimir.",
				Required:            true,
				// Ensures that Terraform destroys and recreates the resource when the namespace changes
				// as the hash(namespace) will change
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"config_yaml": schema.StringAttribute{
				MarkdownDescription: "User supplied namespace's groups rules definition to create in Grafana Mimir as YAML.",
				Required:            true,
				Validators: []validator.String{
					namespaceYAMLValidator{},
				},
			},
			"remote_config_yaml": schema.StringAttribute{
				MarkdownDescription: "The namespace's groups rules definition stored in Grafana Mimir as YAML.",
				Computed:            true,
			},
			"strict_recording_rule_check": schema.BoolAttribute{
				MarkdownDescription: "Fails rules checks that do not match best practices exactly. See: https://prometheus.io/docs/practices/rules/",
				Optional:            true,
				Default:             booldefault.StaticBool(false),
				Computed:            true, // https://discuss.hashicorp.com/t/why-default-attribute-must-also-be-computed/70107/2
			},
			"recording_rule_check": schema.BoolAttribute{
				MarkdownDescription: "Controls whether to run recording rule checks entirely.",
				Optional:            true,
				Default:             booldefault.StaticBool(true),
				Computed:            true, // see above
			},
		},
	}
}

func (r *RulerNamespaceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	tflog.Debug(ctx, "CONFIGURE - init")
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	tflog.Debug(ctx, "CONFIGURE - debug", map[string]interface{}{
		"provider_data": req.ProviderData,
	})

	client, ok := req.ProviderData.(mimirClientInterface)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected mimirClientInterface, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *RulerNamespaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Debug(ctx, "CREATE - init")
	var plan RulerNamespaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.applyRuleGroups(ctx, &plan, "CREATE", &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *RulerNamespaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	tflog.Debug(ctx, "READ - init")
	var state RulerNamespaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	namespace := state.Namespace.ValueString()

	normalized, ok := fetchAndNormalizeRemoteConfigYAML(ctx, r.client, namespace, "READ", &resp.Diagnostics)
	if !ok {
		return
	}

	// config_yaml changes only when the ruler no longer holds what the last
	// apply left there, so the plan re-pushes groups deleted or edited in
	// Mimir. It is left untouched otherwise, to keep the user's formatting out
	// of the plan.
	if !state.ConfigYAML.IsNull() && state.RemoteConfigYAML.ValueString() != "" {
		drifted, err := namespaceDrifted(state.ConfigYAML.ValueString(), state.RemoteConfigYAML.ValueString(), normalized)
		if err != nil {
			resp.Diagnostics.AddError("Error comparing the namespace with Mimir", err.Error())
			return
		}
		if drifted {
			tflog.Info(ctx, "READ: namespace changed in Mimir since the last apply", map[string]interface{}{"namespace": namespace})
			state.ConfigYAML = types.StringValue(normalized)
		}
	}
	state.RemoteConfigYAML = types.StringValue(normalized)
	state.ID = types.StringValue(hash(namespace))
	tflog.Debug(ctx, "Read: setting state.ID", map[string]interface{}{"id": state.ID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *RulerNamespaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Debug(ctx, "DELETE - init")
	var state RulerNamespaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	namespace := state.Namespace.ValueString()
	err := r.client.DeleteNamespace(ctx, namespace)
	if err != nil && !errors.Is(err, client.ErrResourceNotFound) {
		resp.Diagnostics.AddError(
			"Error deleting ruler namespace",
			fmt.Sprintf("deleting ruler namespace (%s): %s", namespace, err),
		)
	}
}

func (r *RulerNamespaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Debug(ctx, "IMPORT STATE - init")
	namespace := req.ID

	var state RulerNamespaceResourceModel
	state.Namespace = types.StringValue(namespace)
	state.ID = types.StringValue(hash(namespace))

	normalized, ok := fetchAndNormalizeRemoteConfigYAML(ctx, r.client, namespace, "IMPORT", &resp.Diagnostics)
	if !ok {
		return
	}
	// Mimir answers 200 for a namespace it does not hold, so a typo in the ID would import nothing.
	groups, err := canonicalGroups(normalized)
	if err != nil {
		resp.Diagnostics.AddError("Error importing ruler namespace", err.Error())
		return
	}
	if len(groups) == 0 {
		resp.Diagnostics.AddError(
			"Error importing ruler namespace",
			fmt.Sprintf("ruler namespace %q not found in Mimir for the configured tenant", namespace),
		)
		return
	}
	state.RemoteConfigYAML = types.StringValue(normalized)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func getRuleNamespaceFromYAML(_ context.Context, configYAML string) (rules.RuleNamespace, error) {
	var ruleNamespace rules.RuleNamespace
	// Same validation scheme and parser options as `mimirtool rules` commands.
	ruleNamespaces, errs := rules.ParseBytes([]byte(configYAML), model.LegacyValidation, mimirtoolUtil.CreatePromQLParser(false))
	if len(errs) > 0 {
		return ruleNamespace, fmt.Errorf("failed to parse namespace definition:\n%s", errors.Join(errs...))
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
	invalidRulesCount := ruleNamespace.CheckRecordingRules(strict, log.NewNopLogger())
	if invalidRulesCount > 0 {
		return fmt.Errorf("namespace contains %d rules that don't match the requirements", invalidRulesCount)
	}
	return nil
}

// Borrowed from https://github.com/grafana/terraform-provider-grafana/blob/main/internal/resources/grafana/resource_dashboard.go
func normalizeNamespaceYAML(config any) (string, int, int, error) {
	configYAML := config.(string)
	var ruleNamespace rules.RuleNamespace

	err := yaml.Unmarshal([]byte(configYAML), &ruleNamespace)
	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to unmarshal YAML config")
	}
	count, mod, _ := ruleNamespace.LintExpressions(rules.MimirBackend, mimirtoolUtil.CreatePromQLParser(false), log.NewNopLogger())

	namespaceBytes, _ := yaml.Marshal(ruleNamespace)
	return string(namespaceBytes), count, mod, err
}

func (r *RulerNamespaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Debug(ctx, "UPDATE - init")
	var plan RulerNamespaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.applyRuleGroups(ctx, &plan, "UPDATE", &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// applyRuleGroups also runs on Create, as the resource manages the whole namespace.
func (r *RulerNamespaceResource) applyRuleGroups(ctx context.Context, plan *RulerNamespaceResourceModel, op string, diagnostics *diag.Diagnostics) bool {
	namespace := plan.Namespace.ValueString()

	ruleNamespace, err := getRuleNamespaceFromYAML(ctx, plan.ConfigYAML.ValueString())
	if err != nil {
		diagnostics.AddError("Failed to parse rule group YAML", err.Error())
		return false
	}

	if plan.RecordingRuleCheck.ValueBool() {
		if err := checkRecordingRules(ruleNamespace, plan.StrictRecordingRuleCheck.ValueBool()); err != nil {
			diagnostics.AddError("Failed to check recording rule group", err.Error())
			return false
		}
	}

	if err := syncRuleGroups(ctx, r.client, namespace, ruleNamespace.Groups); err != nil {
		diagnostics.AddError("Failed to apply rule groups", err.Error())
		return false
	}

	plan.ID = types.StringValue(hash(namespace))

	normalized, ok := fetchAndNormalizeRemoteConfigYAML(ctx, r.client, namespace, op, diagnostics)
	if !ok {
		return false
	}
	plan.RemoteConfigYAML = types.StringValue(normalized)
	return true
}

// shortcut: upstream ListRules alone does not escape the namespace, drop this once it does.
func listNamespaceRules(ctx context.Context, c mimirClientInterface, namespace string) (map[string][]rwrulefmt.RuleGroup, error) {
	return c.ListRules(ctx, url.PathEscape(namespace))
}

// Never delete the namespace first: the ruler would hold none of its groups
// until the rewrite finished, and only part of them if it failed midway.
func syncRuleGroups(ctx context.Context, c mimirClientInterface, namespace string, groups []rwrulefmt.RuleGroup) error {
	remote, err := listNamespaceRules(ctx, c, namespace)
	if err != nil && !errors.Is(err, client.ErrResourceNotFound) {
		return err
	}

	for _, group := range groups {
		if err := c.CreateRuleGroup(ctx, namespace, group); err != nil {
			return err
		}
	}

	configured := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		configured[group.Name] = struct{}{}
	}
	for _, group := range remote[namespace] {
		if _, ok := configured[group.Name]; !ok {
			if err := c.DeleteRuleGroup(ctx, namespace, group.Name); err != nil && !errors.Is(err, client.ErrResourceNotFound) {
				return err
			}
		}
	}
	return nil
}

// Compares what Mimir returned after the last apply with what it returns now,
// never with config_yaml: Mimir drops fields it does not store (a group's
// limit on 2.16), which would read as drift on every plan. Only configured
// groups count; one added in Mimir alone is still removed by the next update,
// as before, but does not trigger it.
func namespaceDrifted(configYAML, appliedRemoteYAML, currentRemoteYAML string) (bool, error) {
	var configured rules.RuleNamespace
	if err := yaml.Unmarshal([]byte(configYAML), &configured); err != nil {
		return false, fmt.Errorf("failed to unmarshal YAML config: %w", err)
	}
	applied, err := canonicalGroups(appliedRemoteYAML)
	if err != nil {
		return false, err
	}
	current, err := canonicalGroups(currentRemoteYAML)
	if err != nil {
		return false, err
	}
	for _, group := range configured.Groups {
		if !reflect.DeepEqual(applied[group.Name], current[group.Name]) {
			return true, nil
		}
	}
	return false, nil
}

// Each group is linted on its own: LintExpressions stops at the first
// expression it cannot parse, which would leave the following groups
// unnormalized depending on what sits before them.
func canonicalGroups(configYAML string) (map[string]any, error) {
	var ruleNamespace rules.RuleNamespace
	if err := yaml.Unmarshal([]byte(configYAML), &ruleNamespace); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML config: %w", err)
	}

	groups := make(map[string]any, len(ruleNamespace.Groups))
	for _, group := range ruleNamespace.Groups {
		single := rules.RuleNamespace{Groups: []rwrulefmt.RuleGroup{group}}
		_, _, _ = single.LintExpressions(rules.MimirBackend, mimirtoolUtil.CreatePromQLParser(false), log.NewNopLogger())

		groupBytes, err := yaml.Marshal(single.Groups[0])
		if err != nil {
			return nil, err
		}
		var decoded any
		if err := yaml.Unmarshal(groupBytes, &decoded); err != nil {
			return nil, err
		}
		groups[group.Name] = decoded
	}
	return groups, nil
}

func fetchAndNormalizeRemoteConfigYAML(
	ctx context.Context,
	client mimirClientInterface,
	namespace string,
	op string,
	diagnostics *diag.Diagnostics,
) (string, bool) {
	normalized, err := readRemoteConfigYAML(ctx, client, namespace, op)
	if err != nil {
		diagnostics.AddError(fmt.Sprintf("Error Reading Mimir RuleGroup after %s", op), err.Error())
		return "", false
	}
	return normalized, true
}

func readRemoteConfigYAML(ctx context.Context, client mimirClientInterface, namespace string, op string) (string, error) {
	remoteNamespaceRuleGroup, err := listNamespaceRules(ctx, client, namespace)
	if err != nil {
		return "", fmt.Errorf("could not read Mimir rulegroup for namespace %q: %w", namespace, err)
	}

	tflog.Trace(ctx, op+": raw value for remoteNamespaceRuleGroup", map[string]interface{}{"remoteNamespaceRuleGroup": remoteNamespaceRuleGroup})

	// Mimir top level key is the namespace name while in the YAML definition the top level key is groups
	// Let's rename the key to be able to have a nice difference
	// TODO: might not be needed anymore since we have introduced the remote_config_yaml attribute
	remoteNamespaceRuleGroup["groups"] = remoteNamespaceRuleGroup[namespace]
	delete(remoteNamespaceRuleGroup, namespace)

	tflog.Debug(ctx, op+": after removing the namespace key", map[string]interface{}{"remoteNamespaceRuleGroupAfterRemovingNamespaceKey": remoteNamespaceRuleGroup})

	remoteConfigYAML, err := yaml.Marshal(remoteNamespaceRuleGroup)
	if err != nil {
		return "", fmt.Errorf("error marshaling rule group YAML: %w", err)
	}
	tflog.Debug(ctx, op+": YAML to be set in state", map[string]interface{}{"remote_config_yaml": remoteConfigYAML})
	normalized, count, mod, err := normalizeNamespaceYAML(string(remoteConfigYAML))
	if err != nil {
		return "", fmt.Errorf("error while normalizing namespace YAML: %w", err)
	}
	tflog.Debug(ctx, op+": results from normalizeNamespaceYAML", map[string]interface{}{"count": count, "mod": mod, "raw": normalized})

	return normalized, nil
}
