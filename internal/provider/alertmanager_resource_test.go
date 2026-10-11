package provider

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccResourceAlertmanager(t *testing.T) {
	tenant := testAccTenant()
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertmanagerDestroy(t, tenant),
		Steps: []resource.TestStep{
			{
				Config: testAccConfig(tenant, testAccResourceAlertmanager),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(
						"mimirtool_alertmanager.demo", "config_yaml", testAccResourceAlertmanagerYaml),
					resource.TestCheckResourceAttr(
						"mimirtool_alertmanager.demo", "templates_config_yaml.default_template", testAccResourceAlertmanagerTemplate),
				),
			},
		},
	})
}

func TestAccResourceAlertmanagerLifecycle(t *testing.T) {
	tenant := testAccTenant()
	resourceName := "mimirtool_alertmanager.demo"
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertmanagerDestroy(t, tenant),
		Steps: []resource.TestStep{
			{
				Config: testAccConfig(tenant, testAccResourceAlertmanager),
			},
			{
				Config: testAccConfig(tenant, testAccResourceAlertmanagerUpdated),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(resourceName, tfjsonpath.New("config_yaml"), knownvalue.StringExact(testAccResourceAlertmanagerUpdatedYaml)),
					statecheck.ExpectKnownValue(resourceName, tfjsonpath.New("templates_config_yaml"), knownvalue.Null()),
				},
			},
			{
				PreConfig: func() {
					testAccRequest(t, tenant, http.MethodPost, testAccAlertmanagerAPI, testAccAlertmanagerOutOfBand)
				},
				Config: testAccConfig(tenant, testAccResourceAlertmanagerUpdated),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(resourceName, tfjsonpath.New("config_yaml"), knownvalue.StringExact(testAccResourceAlertmanagerUpdatedYaml)),
				},
			},
			{
				PreConfig: func() {
					testAccRequest(t, tenant, http.MethodDelete, testAccAlertmanagerAPI, "")
				},
				Config: testAccConfig(tenant, testAccResourceAlertmanagerUpdated),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

func TestAccResourceAlertmanagerNoTemplates(t *testing.T) {
	tenant := testAccTenant()
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertmanagerDestroy(t, tenant),
		Steps: []resource.TestStep{
			{
				Config: testAccConfig(tenant, testAccResourceAlertmanagerNoTemplates),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("mimirtool_alertmanager.demo", tfjsonpath.New("templates_config_yaml"), knownvalue.Null()),
				},
			},
		},
	})
}

func TestAccResourceAlertmanagerEmptyTemplates(t *testing.T) {
	tenant := testAccTenant()
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertmanagerDestroy(t, tenant),
		Steps: []resource.TestStep{
			{
				Config: testAccConfig(tenant, testAccResourceAlertmanagerEmptyTemplates),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("mimirtool_alertmanager.demo", tfjsonpath.New("templates_config_yaml"), knownvalue.MapExact(map[string]knownvalue.Check{})),
				},
			},
		},
	})
}

func TestAccResourceAlertmanagerParseError(t *testing.T) {
	tenant := testAccTenant()
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertmanagerDestroy(t, tenant),
		Steps: []resource.TestStep{
			{
				Config:      testAccConfig(tenant, testAccResourceAlertmanagerParseError),
				ExpectError: regexp.MustCompile(`(?i)invalid yaml syntax|yaml`),
			},
		},
	})
}

func TestAccResourceAlertmanagerImport(t *testing.T) {
	tenant := testAccTenant()
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertmanagerDestroy(t, tenant),
		Steps: []resource.TestStep{
			{
				Config: testAccConfig(tenant, testAccResourceAlertmanager),
			},
			{
				ResourceName:      "mimirtool_alertmanager.demo",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccResourceAlertmanagerUpgradeFromRelease(t *testing.T) {
	for _, release := range testAccReleases {
		for _, tc := range []struct {
			name   string
			config string
			// Releases up to 1.1.0 plan {} -> null on every run without templates.
			releaseHasTemplatesBug bool
		}{
			{"templates", testAccResourceAlertmanager, false},
			{"no_templates", testAccResourceAlertmanagerNoTemplates, true},
		} {
			t.Run(release+"/"+tc.name, func(t *testing.T) {
				tenant := testAccTenant()
				steps := upgradeFromReleaseSteps(release, testAccConfig(tenant, tc.config))
				steps[0].ExpectNonEmptyPlan = tc.releaseHasTemplatesBug
				resource.ParallelTest(t, resource.TestCase{
					CheckDestroy: testAccCheckAlertmanagerDestroy(t, tenant),
					Steps:        steps,
				})
			})
		}
	}
}

const testAccResourceAlertmanagerUpdated = `
resource "mimirtool_alertmanager" "demo" {
  config_yaml = <<YAML
` + testAccResourceAlertmanagerUpdatedYaml + `YAML
}
`

const testAccResourceAlertmanagerUpdatedYaml = `route:
  receiver: updated
receivers:
  - name: updated
`

const testAccAlertmanagerOutOfBand = `alertmanager_config: |
  route:
    receiver: out-of-band
  receivers:
    - name: out-of-band
`

const testAccAlertmanagerAPI = "http://localhost:8080/api/v1/alerts"

func testAccCheckAlertmanagerDestroy(t *testing.T, tenant string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		if status, _ := testAccGet(t, tenant, testAccAlertmanagerAPI); status != http.StatusNotFound {
			return fmt.Errorf("alertmanager config of tenant %s still exists in Mimir: GET returned %d", tenant, status)
		}
		return nil
	}
}

const testAccResourceAlertmanager = `
resource "mimirtool_alertmanager" "demo" {
	config_yaml = file("testdata/example_alertmanager_config.yaml")
	templates_config_yaml = {
	  default_template = file("testdata/example_alertmanager_template.tmpl")
	}
}
`

const testAccResourceAlertmanagerNoTemplates = `
resource "mimirtool_alertmanager" "demo" {
	config_yaml = file("testdata/example_alertmanager_config.yaml")
}
`

const testAccResourceAlertmanagerEmptyTemplates = `
resource "mimirtool_alertmanager" "demo" {
	config_yaml = file("testdata/example_alertmanager_config.yaml")
	templates_config_yaml = {}
}
`

const testAccResourceAlertmanagerYaml = `---
# See: https://grafana.com/docs/mimir/latest/references/http-api/#alertmanager
global:
  smtp_smarthost: 'localhost:25'
  smtp_from: 'youraddress@example.org'
templates:
  - 'default_template'
route:
  receiver: example-email
receivers:
  - name: example-email
    email_configs:
      - to: 'youraddress@example.org'
`
const testAccResourceAlertmanagerTemplate = `{{ define "__alertmanager" }}AlertManager{{ end }}
{{ define "__alertmanagerURL" }}{{ .ExternalURL }}/#/alerts?receiver={{ .Receiver | urlquery }}{{ end }}
`

const testAccResourceAlertmanagerParseError = `
resource "mimirtool_alertmanager" "demo" {
  config_yaml = <<YAML
foo: bar: baz
YAML
}
`
