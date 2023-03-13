package mimirtool

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccResourceNamespace(t *testing.T) {
	for _, useSHA256 := range []bool{false, true} {
		os.Setenv("MIMIR_STORE_RULES_SHA256", fmt.Sprintf("%t", useSHA256))
		defer os.Unsetenv("MIMIR_STORE_RULES_SHA256")

		expectedInitialConfig := testAccResourceNamespaceYaml
		expectedInitialConfigAfterUpdate := testAccResourceNamespaceYamlAfterUpdate
		if useSHA256 {
			expectedInitialConfig = "8ee2cdb65b41d7bdc875ebec6cb1f7a9ab0813e9f6166105a7953d7bf9a68d9b"
			expectedInitialConfigAfterUpdate = "372a400b1eae63b184c036dd2c0aaf71c57e3d8125621e371389d4f7c40c1cb6"
		}
		resource.UnitTest(t, resource.TestCase{
			PreCheck:          func() { testAccPreCheck(t) },
			ProviderFactories: testAccProviderFactories,
			Steps: []resource.TestStep{
				{
					Config: testAccResourceNamespace,
					Check: resource.ComposeTestCheckFunc(
						resource.TestCheckResourceAttr(
							"mimirtool_ruler_namespace.demo", "namespace", "demo"),
						resource.TestCheckResourceAttr(
							"mimirtool_ruler_namespace.demo", "config_yaml", expectedInitialConfig),
					),
				},
				{
					Config: testAccResourceNamespaceAfterUpdate,
					Check: resource.ComposeTestCheckFunc(
						resource.TestCheckResourceAttr(
							"mimirtool_ruler_namespace.demo", "namespace", "demo"),
						resource.TestCheckResourceAttr(
							"mimirtool_ruler_namespace.demo", "config_yaml", expectedInitialConfigAfterUpdate),
					),
				},
			},
		})
	}
}

func TestAccResourceNamespaceCheckRules(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccResourceNamespaceFailsCheck,
				ExpectError: regexp.MustCompile("namespace contains 1 rules that don't match the requirements"),
			},
		},
	})

}

func TestAccResourceNamespaceParseRules(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccResourceNamespaceParseError,
				ExpectError: regexp.MustCompile("field expression not found"),
			},
		},
	})
}

const testAccResourceNamespace = `
resource "mimirtool_ruler_namespace" "demo" {
	namespace = "demo"
	config_yaml = file("testdata/rules.yaml")
  }
`
const testAccResourceNamespaceYaml = `- name: mimir_api_1
  rules:
    - record: cluster_job:cortex_request_duration_seconds:50quantile
      expr: histogram_quantile(0.50, sum(rate(cortex_request_duration_seconds_bucket[1m])) by (le, cluster, job))
    - record: cluster_job:cortex_request_duration_seconds:99quantile
      expr: histogram_quantile(0.99, sum(rate(cortex_request_duration_seconds_bucket[1m])) by (le, cluster, job))
`

const testAccResourceNamespaceAfterUpdate = `
resource "mimirtool_ruler_namespace" "demo" {
	namespace = "demo"
	config_yaml = file("testdata/rules2.yaml")
  }
`
const testAccResourceNamespaceYamlAfterUpdate = `- name: mimir_api_1
  rules:
    - record: cluster_job:cortex_request_duration_seconds:50quantile
      expr: histogram_quantile(0.50, sum(rate(cortex_request_duration_seconds_bucket[1m])) by (le, cluster, job))
    - record: cluster_job:cortex_request_duration_seconds:99quantile
      expr: histogram_quantile(0.99, sum(rate(cortex_request_duration_seconds_bucket[1m])) by (le, cluster, job))
- name: mimir_api_2
  rules:
    - record: cluster_job_route:cortex_request_duration_seconds:99quantile
      expr: histogram_quantile(0.99, sum(rate(cortex_request_duration_seconds_bucket[1m])) by (le, cluster, job, route))
`
const testAccResourceNamespaceFailsCheck = `
resource "mimirtool_ruler_namespace" "demo" {
	namespace = "demo"
	config_yaml = file("testdata/rules-fails-check.yaml")
  }
`

const testAccResourceNamespaceParseError = `
resource "mimirtool_ruler_namespace" "demo" {
	namespace = "demo"
	config_yaml = file("testdata/rules-parse-error.yaml")
  }
`
