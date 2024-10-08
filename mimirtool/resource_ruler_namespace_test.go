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

func TestAccResourceNamespaceRename(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceNamespaceRename,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(
						"mimirtool_ruler_namespace.alerts", "namespace", "alerts_infra"),
				),
			},
			{
				Config: testAccResourceNamespaceRenameAfterUpdate,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(
						"mimirtool_ruler_namespace.alerts", "namespace", "infra"),
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

func TestAccResourceNamespaceQuoting(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceNamespaceQuoting,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(
						"mimirtool_ruler_namespace.demo", "namespace", "demo"),
					resource.TestCheckResourceAttr(
						"mimirtool_ruler_namespace.demo", "config_yaml", testAccResourceNamespaceQuotingExpected),
				),
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
	resource.UnitTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceNamespaceNoCheck,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(
						"mimirtool_ruler_namespace.demo", "namespace", "demo"),
					resource.TestCheckResourceAttr(
						"mimirtool_ruler_namespace.demo", "recording_rule_check", "false"),
					resource.TestCheckResourceAttr(
						"mimirtool_ruler_namespace.demo", "config_yaml", testAccResourceNamespaceNoCheckExpected),
				),
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

const testAccResourceNamespaceRename = `
resource "mimirtool_ruler_namespace" "alerts" {
	namespace = "alerts_infra"
	config_yaml = file("testdata/rules.yaml")
  }
`

const testAccResourceNamespaceRenameAfterUpdate = `
resource "mimirtool_ruler_namespace" "alerts" {
	namespace = "infra"
	config_yaml = file("testdata/rules.yaml")
  }
`

const testAccResourceNamespace = `
resource "mimirtool_ruler_namespace" "demo" {
	namespace = "demo"
	config_yaml = file("testdata/rules.yaml")
  }
`
const testAccResourceNamespaceYaml = `groups:
    - name: mimir_api_1
      rules:
        - record: cluster_job:cortex_request_duration_seconds:99quantile
          expr: histogram_quantile(0.99, sum by (le, cluster, job) (rate(cortex_request_duration_seconds_bucket[1m])))
        - record: cluster_job:cortex_request_duration_seconds:50quantile
          expr: histogram_quantile(0.5, sum by (le, cluster, job) (rate(cortex_request_duration_seconds_bucket[1m])))
`
const testAccResourceNamespaceYamlWhitespace = `groups:
    - name: mimir_api_1
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
const testAccResourceNamespaceYamlAfterUpdate = `groups:
    - name: mimir_api_1
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

const testAccResourceNamespaceNoCheck = `
resource "mimirtool_ruler_namespace" "demo" {
	namespace = "demo"
	config_yaml = file("testdata/rules-fails-check.yaml")
	recording_rule_check = false
  }
`

const testAccResourceNamespaceNoCheckExpected = `groups:
    - name: mimir_api_1
      rules:
        - record: cluster_job_cortex_request_duration_seconds_99quantile
          expr: histogram_quantile(0.99, sum by (le, cluster, job) (rate(cortex_request_duration_seconds_bucket[1m])))
`
const testAccResourceNamespaceParseError = `
resource "mimirtool_ruler_namespace" "demo" {
	namespace = "demo"
	config_yaml = file("testdata/rules-parse-error.yaml")
  }
`
const testAccResourceNamespaceQuoting = `
resource "mimirtool_ruler_namespace" "demo" {
	namespace = "demo"
	config_yaml = file("testdata/rules-quoting.yaml")
  }
`
const testAccResourceNamespaceQuotingExpected = `groups:
    - name: NodeExporter
      rules:
        - alert: HostDiskWillFillIn24Hours
          expr: (node_filesystem_avail_bytes * 100) / node_filesystem_size_bytes < 10 and on (instance, device, mountpoint) predict_linear(node_filesystem_avail_bytes{fstype!~"tmpfs"}[1h], 24 * 3600) < 0 and on (instance, device, mountpoint) node_filesystem_readonly == 0
          for: 2m
          labels:
            severity: warning
          annotations:
            description: |-
                Filesystem is predicted to run out of space within the next 24 hours at current write rate
                  VALUE = {{ $value }}
                  LABELS = {{ $labels }}
            summary: Host disk will fill in 24 hours (instance {{ $labels.instance }})
        - alert: HostHighCpuLoad
          expr: 100 - (avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[2m])) * 100) > 80
          labels:
            severity: warning
          annotations:
            description: |-
                CPU load is > 80%
                  VALUE = {{ $value }}
                  LABELS = {{ $labels }}
            summary: Host high CPU load (instance {{ $labels.instance }})
`
