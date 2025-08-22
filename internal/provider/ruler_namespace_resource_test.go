// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"reflect"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"gopkg.in/yaml.v3"
)

// Structs for semantic YAML comparison
// These should match the structure of your rule YAML
// Add fields as needed for your use case

type Rule struct {
	Record string `yaml:"record,omitempty"`
	Expr   string `yaml:"expr,omitempty"`
}
type RuleGroup struct {
	Name  string `yaml:"name"`
	Rules []Rule `yaml:"rules"`
}
type RuleNamespace struct {
	Groups []RuleGroup `yaml:"groups"`
}

// Semantic YAML matcher for knownvalue.StringFunc
func SemanticYAMLMatcher(expected string) func(string) error {
	return func(actual string) error {
		var expectedObj, actualObj RuleNamespace
		if err := yaml.Unmarshal([]byte(expected), &expectedObj); err != nil {
			return fmt.Errorf("Failed to parse expected YAML: %s", err)
		}
		if err := yaml.Unmarshal([]byte(actual), &actualObj); err != nil {
			return fmt.Errorf("Failed to parse actual YAML: %s", err)
		}
		if !reflect.DeepEqual(expectedObj, actualObj) {
			return fmt.Errorf("YAML semantic mismatch\nExpected: %#v\nActual: %#v", expectedObj, actualObj)
		}
		return nil
	}
}

// SemanticYAMLStateCheck returns a statecheck.StateCheck for semantic YAML comparison on a given resource and attribute.
func SemanticYAMLStateCheck(resourceName, attr, expected string) statecheck.StateCheck {
	return statecheck.ExpectKnownValue(
		resourceName,
		tfjsonpath.New(attr),
		knownvalue.StringFunc(SemanticYAMLMatcher(expected)),
	)
}

func TestAccResourceNamespace(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceNamespace,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mimirtool_ruler_namespace.demo", "namespace", "demo"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					SemanticYAMLStateCheck("mimirtool_ruler_namespace.demo", "remote_config_yaml", testAccResourceNamespaceYaml),
				},
			},
			{
				Config: testAccResourceNamespaceAfterUpdate,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"mimirtool_ruler_namespace.demo",
						tfjsonpath.New("namespace"),
						knownvalue.StringExact("demo"),
					),
					SemanticYAMLStateCheck("mimirtool_ruler_namespace.demo", "remote_config_yaml", testAccResourceNamespaceYamlAfterUpdate),
				},
			},
			{
				ResourceName:      "mimirtool_ruler_namespace.demo",
				ImportStateId:     "demo",
				ImportState:       true,
				ImportStateVerify: true,
				// These fields can't be retrieved from mimir ruler
				ImportStateVerifyIgnore: []string{"recording_rule_check", "strict_recording_rule_check", "config_yaml"},
			},
		},
	})
}

func TestAccResourceNamespaceRename(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceNamespaceRename,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"mimirtool_ruler_namespace.alerts",
						tfjsonpath.New("namespace"),
						knownvalue.StringExact("alerts_infra"),
					),
				},
			},
			{
				Config: testAccResourceNamespaceRenameAfterUpdate,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"mimirtool_ruler_namespace.alerts",
						tfjsonpath.New("namespace"),
						knownvalue.StringExact("infra"),
					),
				},
			},
		},
	})
}

func TestAccResourceNamespaceDiffSuppress(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceNamespaceWhitespaceDiff,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"mimirtool_ruler_namespace.demo",
						tfjsonpath.New("namespace"),
						knownvalue.StringExact("demo"),
					),
					SemanticYAMLStateCheck("mimirtool_ruler_namespace.demo", "remote_config_yaml", testAccResourceNamespaceYamlWhitespace),
				},
			},
		},
	})
}

func TestAccResourceNamespaceQuoting(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceNamespaceQuoting,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"mimirtool_ruler_namespace.demo",
						tfjsonpath.New("namespace"),
						knownvalue.StringExact("demo"),
					),
					SemanticYAMLStateCheck("mimirtool_ruler_namespace.demo", "remote_config_yaml", testAccResourceNamespaceQuotingExpected),
				},
			},
		},
	})
}

func TestAccResourceNamespaceCheckRules(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccResourceNamespaceFailsCheck,
				ExpectError: regexp.MustCompile("namespace contains 1 rules that don't match the requirements"),
			},
		},
	})
}

func TestAccResourceNamespaceNoCheck(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceNamespaceNoCheck,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"mimirtool_ruler_namespace.demo",
						tfjsonpath.New("namespace"),
						knownvalue.StringExact("demo"),
					),
					statecheck.ExpectKnownValue(
						"mimirtool_ruler_namespace.demo",
						tfjsonpath.New("recording_rule_check"),
						knownvalue.Bool(false),
					),
					SemanticYAMLStateCheck("mimirtool_ruler_namespace.demo", "remote_config_yaml", testAccResourceNamespaceNoCheckExpected),
				},
			},
		},
	})
}

func TestAccResourceNamespaceParseRules(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccResourceNamespaceParseError,
				ExpectError: regexp.MustCompile("field expression not found"),
			},
		},
	})
}

const testAccResourceNamespaceRename = `
provider "mimirtool" {
  address = "http://localhost:8080"
}

resource "mimirtool_ruler_namespace" "alerts" {
	namespace = "alerts_infra"
	config_yaml = file("testdata/rules.yaml")
  }
`

const testAccResourceNamespaceRenameAfterUpdate = `
provider "mimirtool" {
  address = "http://localhost:8080"
}

resource "mimirtool_ruler_namespace" "alerts" {
	namespace = "infra"
	config_yaml = file("testdata/rules.yaml")
  }
`

const testAccResourceNamespace = `
provider "mimirtool" {
  address = "http://localhost:8080"
}

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
          expr: histogram_quantile(0.5, sum by (le, cluster, job) (rate(cortex_request_duration_seconds_bucket[1m])))`

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
provider "mimirtool" {
  address = "http://localhost:8080"
}

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

const testAccResourceNamespaceWhitespaceDiff = `provider "mimirtool" {
  address = "http://localhost:8080"
}

resource "mimirtool_ruler_namespace" "demo" {
	namespace = "demo"
	config_yaml = file("testdata/rules2_spacing.yaml")
  }
`
const testAccResourceNamespaceFailsCheck = `
provider "mimirtool" {
  address = "http://localhost:8080"
}

resource "mimirtool_ruler_namespace" "demo" {
	namespace = "demo"
	config_yaml = file("testdata/rules-fails-check.yaml")
  }
`

const testAccResourceNamespaceNoCheck = `
provider "mimirtool" {
  address = "http://localhost:8080"
}

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
provider "mimirtool" {
  address = "http://localhost:8080"
}

resource "mimirtool_ruler_namespace" "demo" {
	namespace = "demo"
	config_yaml = file("testdata/rules-parse-error.yaml")
  }
`
const testAccResourceNamespaceQuoting = `
provider "mimirtool" {
  address = "http://localhost:8080"
}

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
