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
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"go.yaml.in/yaml/v3"
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
	tenant := testAccTenant()
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRulerNamespaceDestroy(t, tenant),
		Steps: []resource.TestStep{
			{
				Config: testAccConfig(tenant, testAccResourceNamespace),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mimirtool_ruler_namespace.demo", "namespace", "demo"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					SemanticYAMLStateCheck("mimirtool_ruler_namespace.demo", "remote_config_yaml", testAccResourceNamespaceYaml),
				},
			},
			{
				Config: testAccConfig(tenant, testAccResourceNamespaceAfterUpdate),
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
			{
				ResourceName:    "mimirtool_ruler_namespace.demo",
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithID,
				ImportStateId:   "demo",
				// config_yaml is compared as text, so the first plan after an import
				// pushes the same groups again. It must never replace the namespace.
				ExpectNonEmptyPlan: true,
				ImportPlanChecks: resource.ImportPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("mimirtool_ruler_namespace.demo", plancheck.ResourceActionUpdate),
					},
				},
			},
		},
	})
}

func TestAccResourceNamespaceImportMissing(t *testing.T) {
	tenant := testAccTenant()
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:        testAccConfig(tenant, testAccResourceNamespace),
				ResourceName:  "mimirtool_ruler_namespace.demo",
				ImportState:   true,
				ImportStateId: "demo",
				ExpectError:   regexp.MustCompile(`ruler namespace \(demo\) not found`),
			},
		},
	})
}

func TestAccResourceNamespaceRename(t *testing.T) {
	tenant := testAccTenant()
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRulerNamespaceDestroy(t, tenant),
		Steps: []resource.TestStep{
			{
				Config: testAccConfig(tenant, testAccResourceNamespaceRename),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"mimirtool_ruler_namespace.alerts",
						tfjsonpath.New("namespace"),
						knownvalue.StringExact("alerts_infra"),
					),
				},
			},
			{
				Config: testAccConfig(tenant, testAccResourceNamespaceRenameAfterUpdate),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("mimirtool_ruler_namespace.alerts", plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"mimirtool_ruler_namespace.alerts",
						tfjsonpath.New("namespace"),
						knownvalue.StringExact("infra"),
					),
					rulerNamespaceInMimir{t: t, tenant: tenant, namespace: "alerts_infra", want: false},
					rulerNamespaceInMimir{t: t, tenant: tenant, namespace: "infra", want: true},
				},
			},
		},
	})
}

func TestAccResourceNamespaceDiffSuppress(t *testing.T) {
	tenant := testAccTenant()
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRulerNamespaceDestroy(t, tenant),
		Steps: []resource.TestStep{
			{
				Config: testAccConfig(tenant, testAccResourceNamespaceWhitespaceDiff),
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
	tenant := testAccTenant()
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRulerNamespaceDestroy(t, tenant),
		Steps: []resource.TestStep{
			{
				Config: testAccConfig(tenant, testAccResourceNamespaceQuoting),
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
	tenant := testAccTenant()
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRulerNamespaceDestroy(t, tenant),
		Steps: []resource.TestStep{
			{
				Config:      testAccConfig(tenant, testAccResourceNamespaceFailsCheck),
				ExpectError: regexp.MustCompile(`(?s)Error creating ruler namespace.*checking recording rules of ruler\s+namespace\s+\(demo\):\s+namespace\s+contains\s+1\s+rules`),
			},
		},
	})
}

func TestAccResourceNamespaceNoCheck(t *testing.T) {
	tenant := testAccTenant()
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRulerNamespaceDestroy(t, tenant),
		Steps: []resource.TestStep{
			{
				Config: testAccConfig(tenant, testAccResourceNamespaceNoCheck),
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
	tenant := testAccTenant()
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRulerNamespaceDestroy(t, tenant),
		Steps: []resource.TestStep{
			{
				Config:      testAccConfig(tenant, testAccResourceNamespaceParseError),
				ExpectError: regexp.MustCompile("field expression not found"),
			},
		},
	})
}

func TestAccResourceNamespaceRemoteConfigReadOnly(t *testing.T) {
	tenant := testAccTenant()
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRulerNamespaceDestroy(t, tenant),
		Steps: []resource.TestStep{
			{
				Config:      testAccConfig(tenant, testAccResourceNamespaceRemoteConfigSet),
				ExpectError: regexp.MustCompile("Invalid Configuration for Read-Only Attribute"),
			},
		},
	})
}

const testAccResourceNamespaceRemoteConfigSet = `
resource "mimirtool_ruler_namespace" "demo" {
	namespace = "demo"
	config_yaml = file("testdata/rules.yaml")
	remote_config_yaml = file("testdata/rules.yaml")
  }
`

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

func TestAccResourceNamespaceTenantIsolation(t *testing.T) {
	tenant, other := testAccTenant(), testAccTenant()
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRulerNamespaceDestroy(t, tenant),
		Steps: []resource.TestStep{
			{
				Config: testAccConfig(tenant, testAccResourceNamespace),
				ConfigStateChecks: []statecheck.StateCheck{
					rulerNamespaceInMimir{t: t, tenant: tenant, namespace: "demo", want: true},
					rulerNamespaceInMimir{t: t, tenant: other, namespace: "demo", want: false},
				},
			},
		},
	})
}

func TestAccResourceNamespaceEmptyName(t *testing.T) {
	tenant := testAccTenant()
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccConfig(tenant, fmt.Sprintf(testAccResourceNamespaceNamed, "")),
				ExpectError: regexp.MustCompile(`Attribute namespace must not be empty`),
			},
		},
	})
}

func TestAccResourceNamespaceSpecialCharacters(t *testing.T) {
	tenant := testAccTenant()
	namespace := "team a/rules?v=1"
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckRulerNamespaceDestroy(t, tenant),
		Steps: []resource.TestStep{
			{
				Config: testAccConfig(tenant, fmt.Sprintf(testAccResourceNamespaceNamed, namespace)),
				ConfigStateChecks: []statecheck.StateCheck{
					SemanticYAMLStateCheck("mimirtool_ruler_namespace.demo", "remote_config_yaml", testAccResourceNamespaceYaml),
				},
			},
			{
				ResourceName:            "mimirtool_ruler_namespace.demo",
				ImportState:             true,
				ImportStateId:           namespace,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"recording_rule_check", "strict_recording_rule_check", "config_yaml"},
			},
		},
	})
}

const testAccResourceNamespaceNamed = `
resource "mimirtool_ruler_namespace" "demo" {
  namespace   = %q
  config_yaml = file("testdata/rules.yaml")
}
`
