// This file implements the Terraform resource for managing the Alertmanager configuration in Grafana Mimir.
// It supports create, read, update, and delete operations, and handles templates as well as config YAML.
// See: https://grafana.com/docs/mimir/latest/references/http-api/#alertmanager

package provider

import (
	"context"
	"fmt"

	"errors"

	"github.com/grafana/mimir/pkg/mimirtool/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &AlertmanagerResource{}
	_ resource.ResourceWithImportState = &AlertmanagerResource{}
)

func NewAlertmanagerResource() resource.Resource {
	return &AlertmanagerResource{}
}

// AlertmanagerResource defines the resource implementation.
type AlertmanagerResource struct {
	client mimirClientInterface
}

func (r *AlertmanagerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alertmanager"
}

func (r *AlertmanagerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the Alertmanager configuration in Grafana Mimir. [Official documentation](https://grafana.com/docs/mimir/latest/references/http-api/#alertmanager)",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID for the Alertmanager resource (always 'alertmanager'). This is a singleton resource per tenant.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"config_yaml": schema.StringAttribute{
				MarkdownDescription: "The Alertmanager configuration to load in Grafana Mimir as YAML. This should be a valid Alertmanager YAML config.",
				Required:            true,
				Validators: []validator.String{
					yamlSyntaxValidator{},
				},
			},
			"templates_config_yaml": schema.MapAttribute{
				MarkdownDescription: "A map of template names to template YAML content to load along with the Alertmanager configuration.",
				ElementType:         types.StringType,
				Optional:            true,
			},
		},
	}
}

func (r *AlertmanagerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
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

type AlertmanagerResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	ConfigYAML          types.String `tfsdk:"config_yaml"`
	TemplatesConfigYAML types.Map    `tfsdk:"templates_config_yaml"`
}

func (r *AlertmanagerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AlertmanagerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	alertmanagerConfig := plan.ConfigYAML.ValueString()
	templates := mapStringFromTypesMap(plan.TemplatesConfigYAML)

	err := r.client.CreateAlertmanagerConfig(ctx, alertmanagerConfig, templates)
	if err != nil {
		tflog.Error(ctx, "Failed to create Alertmanager config via POST", map[string]interface{}{"error": err})
		resp.Diagnostics.AddError(
			"Error creating Alertmanager config",
			fmt.Sprintf("Failed to create Alertmanager config via POST: %s", err),
		)
		return
	}

	plan.ID = types.StringValue("alertmanager")
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AlertmanagerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state AlertmanagerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	alertmanagerConfig, templates, err := r.client.GetAlertmanagerConfig(ctx)
	if err != nil {
		if errors.Is(err, client.ErrResourceNotFound) {
			tflog.Info(ctx, "No alertmanager config found in backend; removing from state")
			resp.State.RemoveResource(ctx)
			return
		}
		tflog.Error(ctx, "Failed to read Alertmanager config", map[string]interface{}{"error": err})
		resp.Diagnostics.AddError(
			"Error reading Alertmanager config",
			fmt.Sprintf("Failed to read Alertmanager config: %s", err),
		)
		return
	}

	state.ConfigYAML = types.StringValue(alertmanagerConfig)
	state.TemplatesConfigYAML = typeMapFromMapString(templates)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// The backend API does not support PUT for Alertmanager config updates.
// Therefore, Update uses the same logic as Create (POST) to replace the configuration.
func (r *AlertmanagerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan AlertmanagerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	alertmanagerConfig := plan.ConfigYAML.ValueString()
	templates := mapStringFromTypesMap(plan.TemplatesConfigYAML)

	err := r.client.CreateAlertmanagerConfig(ctx, alertmanagerConfig, templates)
	if err != nil {
		tflog.Error(ctx, "Failed to update Alertmanager config via POST", map[string]interface{}{"error": err})
		resp.Diagnostics.AddError(
			"Error updating Alertmanager config",
			fmt.Sprintf("Failed to update Alertmanager config via POST: %s", err),
		)
		return
	}

	plan.ID = types.StringValue("alertmanager")
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AlertmanagerResource) Delete(ctx context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	err := r.client.DeleteAlermanagerConfig(ctx)
	if err != nil {
		tflog.Error(ctx, "Failed to delete Alertmanager config", map[string]interface{}{"error": err})
		resp.Diagnostics.AddError(
			"Error deleting Alertmanager config",
			fmt.Sprintf("Failed to delete Alertmanager config: %s", err),
		)
		return
	}
	resp.State.RemoveResource(ctx)
}

func (r *AlertmanagerResource) ImportState(ctx context.Context, _ resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), "alertmanager")...)
}
