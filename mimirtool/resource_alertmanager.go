package mimirtool

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	mimirtool "github.com/grafana/mimir/pkg/mimirtool/client"
)

func resourceAlertManager() *schema.Resource {
	return &schema.Resource{
		Description: `
[Official documentation](https://grafana.com/docs/mimir/latest/references/http-api/#alertmanager)
`,

		CreateContext: alertmanagerCreate,
		ReadContext:   alertmanagerRead,
		UpdateContext: alertmanagerCreate, // There is no PUT, the POST is responsible to overwrite the configuration
		DeleteContext: alertmanagerDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"config_yaml": {
				Description: "The Alertmanager configuration to load in Grafana Mimir as YAML.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"templates_config_yaml": {
				Description: "The templates to load along with the configuration.",
				Type:        schema.TypeMap,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Optional:    true,
			},
		},
	}
}

func alertmanagerCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	client := meta.(*client).cli
	alertmanagerConfig := d.Get("config_yaml").(string)
	templatesMap := d.Get("templates_config_yaml").(map[string]interface{})

	templates := stringValueMap(templatesMap)

	err := client.CreateAlertmanagerConfig(ctx, alertmanagerConfig, templates)
	if err != nil {
		return diag.FromErr(err)
	}
	// Mimir supports only one alertmanager configuration per tenant as such there is no associated ID
	d.SetId("alertmanager")
	return alertmanagerRead(ctx, d, meta)
}

func alertmanagerRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	client := meta.(*client).cli
	alertmanagerConfig, templates, err := client.GetAlertmanagerConfig(ctx)
	if errors.Is(err, mimirtool.ErrResourceNotFound) {
		// need to tell terraform the resource does not exist
		tflog.Info(ctx, "No alertmanager mimir side")
		d.SetId("")
	} else if err != nil {
		return diag.FromErr(err)
	}
	d.Set("config_yaml", alertmanagerConfig)
	d.Set("templates_config_yaml", templates)
	return nil
}

func alertmanagerDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	var diags diag.Diagnostics
	client := meta.(*client).cli
	err := client.DeleteAlermanagerConfig(ctx)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return diags
}
