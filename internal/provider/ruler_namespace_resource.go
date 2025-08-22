package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/grafana/mimir/pkg/mimirtool/client"
	"github.com/grafana/mimir/pkg/mimirtool/rules"
	"github.com/grafana/mimir/pkg/mimirtool/rules/rwrulefmt"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"gopkg.in/yaml.v3"
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
	client *client.MimirClient
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
		MarkdownDescription: "[Official documentation](https://grafana.com/docs/mimir/latest/references/http-api/#ruler)",

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
				Optional:            true,
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

	client, ok := req.ProviderData.(*client.MimirClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.MimirClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *RulerNamespaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	tflog.Debug(ctx, "CREATE - init")
	var plan RulerNamespaceResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Extract values from the plan
	namespace := plan.Namespace.ValueString()
	ruleGroup := plan.ConfigYAML.ValueString()
	strictRecordingRuleCheck := plan.StrictRecordingRuleCheck.ValueBool()
	recordingRuleCheck := plan.RecordingRuleCheck.ValueBool()

	tflog.Debug(ctx, "CREATE - values from plan", map[string]interface{}{
		"namespace":                namespace,
		"ruleGroup":                ruleGroup,
		"strictRecordingRuleCheck": strictRecordingRuleCheck,
		"recordingRuleCheck":       recordingRuleCheck,
	})

	// Parse YAML
	ruleNamespace, err := getRuleNamespaceFromYAML(ctx, ruleGroup)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to parse rule group YAML",
			err.Error(),
		)
		return
	}

	if recordingRuleCheck {
		err = checkRecordingRules(ruleNamespace, strictRecordingRuleCheck)
		if err != nil {
			// TODO: add a more explicit error message
			resp.Diagnostics.AddError(
				"Failed to check recording rule group",
				err.Error(),
			)
			return
		}
	}

	// Create rule groups in Mimir
	if err := createAllRuleGroups(ctx, r.client, namespace, ruleNamespace.Groups); err != nil {
		resp.Diagnostics.AddError(
			"Failed to create rule groups",
			err.Error(),
		)
		return
	}

	// Set ID
	plan.ID = types.StringValue(hash(namespace))

	// Always fetch canonical YAML from backend and store in state
	normalized, ok := fetchAndNormalizeRemoteConfigYAML(ctx, r.client, namespace, "CREATE", &resp.Diagnostics)
	if !ok {
		return
	}
	plan.RemoteConfigYAML = types.StringValue(normalized)

	// Save the state
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

	// Use the same helper as Create/Update for fetching and normalizing YAML
	normalized, ok := fetchAndNormalizeRemoteConfigYAML(ctx, r.client, namespace, "READ", &resp.Diagnostics)
	if !ok {
		return
	}
	state.RemoteConfigYAML = types.StringValue(normalized)
	state.ID = types.StringValue(hash(namespace))
	tflog.Debug(ctx, "Read: setting state.ID", map[string]interface{}{"id": state.ID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *RulerNamespaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Debug(ctx, "DELETE - init")
	var state RulerNamespaceResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	// Extract namespace from state
	namespace := state.Namespace.ValueString()

	tflog.Debug(ctx, "DELETE - values from state", map[string]interface{}{
		"state_config_yaml": state.ConfigYAML.ValueString(),
	})

	err := r.client.DeleteNamespace(ctx, namespace)

	if err != nil && !strings.Contains(err.Error(), "not found") {
		resp.Diagnostics.AddError(
			"Unable to Delete Resource",
			"An unexpected error occurred while attempting to delete the resource. "+
				"Please retry the operation or report this issue to the provider developers.\n\n"+
				"HTTP Error: "+err.Error(),
		)

		return
	}

	// If the logic reaches here, it implicitly succeeded and will remove
	// the resource from state if there are no other errors.
}

func (r *RulerNamespaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Debug(ctx, "IMPORT STATE - init")
	// The import ID is the namespace name
	namespace := req.ID

	// Create a state with the namespace set
	var state RulerNamespaceResourceModel
	state.Namespace = types.StringValue(namespace)
	state.ID = types.StringValue(hash(namespace))

	// Fetch backend rules to update the state
	normalized, ok := fetchAndNormalizeRemoteConfigYAML(ctx, r.client, namespace, "IMPORT", &resp.Diagnostics)
	if !ok {
		return
	}
	state.RemoteConfigYAML = types.StringValue(normalized)
	// state.ConfigYAML = types.StringValue(normalized) // Set config_yaml to the same value for import

	// Set the state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func getRuleNamespaceFromYAML(_ context.Context, configYAML string) (rules.RuleNamespace, error) {
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

// Borrowed from https://github.com/grafana/terraform-provider-grafana/blob/main/internal/resources/grafana/resource_dashboard.go
func normalizeNamespaceYAML(config any) (string, int, int, error) {
	configYAML := config.(string)
	var ruleNamespace rules.RuleNamespace

	err := yaml.Unmarshal([]byte(configYAML), &ruleNamespace)
	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to unmarshal YAML config")
	}
	count, mod, _ := ruleNamespace.LintExpressions(rules.MimirBackend)

	namespaceBytes, _ := yaml.Marshal(ruleNamespace)
	return string(namespaceBytes), count, mod, err
}

func (r *RulerNamespaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Debug(ctx, "UPDATE - init")
	var plan RulerNamespaceResourceModel
	var state RulerNamespaceResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	namespace := plan.Namespace.ValueString()
	ruleGroup := plan.ConfigYAML.ValueString()
	strictRecordingRuleCheck := plan.StrictRecordingRuleCheck.ValueBool()
	recordingRuleCheck := plan.RecordingRuleCheck.ValueBool()

	// Delete the current namespace to replace it
	err := r.client.DeleteNamespace(ctx, namespace)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to delete existing namespace",
			err.Error(),
		)
		return
	}

	ruleNamespace, err := getRuleNamespaceFromYAML(ctx, ruleGroup)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to parse rule group YAML",
			err.Error(),
		)
		return
	}

	if recordingRuleCheck {
		err = checkRecordingRules(ruleNamespace, strictRecordingRuleCheck)
		if err != nil {
			resp.Diagnostics.AddError(
				"Failed to check recording rule group",
				err.Error(),
			)
			return
		}
	}

	// Create all rule groups for the namespace
	if err := createAllRuleGroups(ctx, r.client, namespace, ruleNamespace.Groups); err != nil {
		resp.Diagnostics.AddError(
			"Failed to create rule groups",
			err.Error(),
		)
		return
	}

	// Set the ID
	plan.ID = types.StringValue(hash(namespace))

	// Fetch backend rules
	normalized, ok := fetchAndNormalizeRemoteConfigYAML(ctx, r.client, namespace, "UPDATE", &resp.Diagnostics)
	if !ok {
		return
	}
	plan.RemoteConfigYAML = types.StringValue(normalized)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Create rule groups in Mimir
func createAllRuleGroups(ctx context.Context, client *client.MimirClient, namespace string, groups []rwrulefmt.RuleGroup) error {
	for _, group := range groups {
		if err := client.CreateRuleGroup(ctx, namespace, group); err != nil {
			return err
		}
	}
	return nil
}

// Helper function for fetching and normalizing the remote config YAML
func fetchAndNormalizeRemoteConfigYAML(
	ctx context.Context,
	client *client.MimirClient,
	namespace string,
	op string,
	diagnostics *diag.Diagnostics,
) (string, bool) {
	remoteNamespaceRuleGroup, err := client.ListRules(ctx, namespace)
	if err != nil {
		diagnostics.AddError(
			fmt.Sprintf("Error Reading Mimir RuleGroup after %s", op),
			fmt.Sprintf("Could not read Mimir rulegroup for namespace %q: %s", namespace, err.Error()),
		)
		return "", false
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
		diagnostics.AddError(
			fmt.Sprintf("Error marshaling rule group YAML after %s", op),
			err.Error(),
		)
		return "", false
	}
	tflog.Debug(ctx, op+": YAML to be set in state", map[string]interface{}{"remote_config_yaml": remoteConfigYAML})
	normalized, count, mod, err := normalizeNamespaceYAML(string(remoteConfigYAML))
	if err != nil {
		diagnostics.AddError(
			fmt.Sprintf("Error while normalizing namespace YAML after %s", op),
			err.Error(),
		)
		return "", false
	}
	tflog.Debug(ctx, op+": results from normalizeNamespaceYAML", map[string]interface{}{"count": count, "mod": mod, "raw": normalized})

	return normalized, true
}
