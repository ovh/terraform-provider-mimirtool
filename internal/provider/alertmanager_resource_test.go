package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccResourceAlertmanager(t *testing.T) {
	tenant := testAccTenant()
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
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

func TestAccResourceAlertmanagerNoTemplates(t *testing.T) {
	tenant := testAccTenant()
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
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
