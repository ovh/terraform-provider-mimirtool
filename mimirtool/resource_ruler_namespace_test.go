package mimirtool

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccResourceNamespace(t *testing.T) {
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
						"mimirtool_ruler_namespace.demo", "config_yaml", testAccResourceNamespaceYaml),
				),
			},
			{
				Config: testAccResourceNamespaceAfterUpdate,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(
						"mimirtool_ruler_namespace.demo", "namespace", "demo"),
					resource.TestCheckResourceAttr(
						"mimirtool_ruler_namespace.demo", "config_yaml", testAccResourceNamespaceYamlAfterUpdate),
				),
			},
		},
	})
}

func TestAccResourceNamespaceDiffSuppress(t *testing.T) {

	resource.UnitTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceNamespaceWhitespaceDiff,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(
						"mimirtool_ruler_namespace.demo", "namespace", "demo"),
					resource.TestCheckResourceAttr(
						"mimirtool_ruler_namespace.demo", "config_yaml", testAccResourceNamespaceYamlWhitespace),
				),
			},
		},
	})
}

			},
		},
	})
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
    - record: cluster_job:cortex_request_duration_seconds:99quantile
      expr: histogram_quantile(0.99, sum by (le, cluster, job) (rate(cortex_request_duration_seconds_bucket[1m])))
    - record: cluster_job:cortex_request_duration_seconds:50quantile
      expr: histogram_quantile(0.5, sum by (le, cluster, job) (rate(cortex_request_duration_seconds_bucket[1m])))
`
const testAccResourceNamespaceYamlWhitespace = `- name: mimir_api_1
  rules:
    - record: cluster_job:cortex_request_duration_seconds:99quantile
      expr: |-
        histogram_quantile(0.99, sum by (le, cluster, job) (rate(cortex_request_duration_seconds_bucket[1m])))
    - record: cluster_job:cortex_request_duration_seconds:50quantile
      expr: histogram_quantile(0.5, sum by (le, cluster, job) (rate(cortex_request_duration_seconds_bucket[1m])))
`
const testAccResourceNamespaceAfterUpdate = `
resource "mimirtool_ruler_namespace" "demo" {
	namespace = "demo"
	config_yaml = file("testdata/rules2.yaml")
  }
`
const testAccResourceNamespaceYamlAfterUpdate = `- name: mimir_api_1
  rules:
    - record: cluster_job:cortex_request_duration_seconds:99quantile
      expr: histogram_quantile(0.99, sum by (le, cluster, job) (rate(cortex_request_duration_seconds_bucket[1m])))
    - record: cluster_job:cortex_request_duration_seconds:50quantile
      expr: histogram_quantile(0.5, sum by (le, cluster, job) (rate(cortex_request_duration_seconds_bucket[1m])))
- name: mimir_api_2
  rules:
    - record: cluster_job_route:cortex_request_duration_seconds:99quantile
      expr: histogram_quantile(0.99, sum by (le, cluster, job, route) (rate(cortex_request_duration_seconds_bucket[1m])))
`
const testAccResourceNamespaceWhitespaceDiff = `
resource "mimirtool_ruler_namespace" "demo" {
	namespace = "demo"
	config_yaml = file("testdata/rules2_spacing.yaml")
  }
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
