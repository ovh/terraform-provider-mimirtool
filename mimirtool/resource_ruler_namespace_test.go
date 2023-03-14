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
			expectedInitialConfig = "a90a22389a8e736469aa2c70145ca4d3481c5f6565423fef484f140541eec113"
			expectedInitialConfigAfterUpdate = "fce4306cdc615aeb3c04385ff7b565cbbe77453758894f898e4651576510f883"
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

func TestAccResourceNamespaceDiffSuppress(t *testing.T) {
	for _, useSHA256 := range []bool{false, true} {
		os.Setenv("MIMIR_STORE_RULES_SHA256", fmt.Sprintf("%t", useSHA256))
		defer os.Unsetenv("MIMIR_STORE_RULES_SHA256")

		var expected string
		if !useSHA256 {
			expected = testAccResourceNamespaceYamlWhitespace
		} else {
			expected = "f9a92a1e50895f6c0e626a2c8b0a8c4f6c1211e9d1089e73163b08c366a8dfc4"
		}

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
							"mimirtool_ruler_namespace.demo", "config_yaml", expected),
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
