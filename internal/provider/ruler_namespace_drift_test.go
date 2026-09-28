// Copyright 2026 OVHcloud
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
)

const testAccRulerAPI = "http://localhost:8080/prometheus/config/v1/rules/"

// Out-of-band changes are made straight through the ruler API, the way an
// operator or another tool would, so the provider only discovers them on Read.
func rulerRequest(t *testing.T, method, path, body string) {
	t.Helper()
	req, err := http.NewRequest(method, testAccRulerAPI+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/yaml")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		t.Fatalf("%s %s: unexpected status %d", method, path, res.StatusCode)
	}
}

func driftRepairStep(preConfig func()) resource.TestStep {
	return resource.TestStep{
		PreConfig: preConfig,
		Config:    testAccResourceNamespaceDrift,
		ConfigPlanChecks: resource.ConfigPlanChecks{
			PreApply: []plancheck.PlanCheck{
				plancheck.ExpectResourceAction("mimirtool_ruler_namespace.drift", plancheck.ResourceActionUpdate),
			},
		},
		ConfigStateChecks: []statecheck.StateCheck{
			SemanticYAMLStateCheck("mimirtool_ruler_namespace.drift", "remote_config_yaml", testAccResourceNamespaceDriftExpected),
		},
	}
}

func TestAccResourceNamespaceDrift(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceNamespaceDrift,
			},
			driftRepairStep(func() {
				rulerRequest(t, http.MethodDelete, "drift/mimir_api_2", "")
			}),
			driftRepairStep(func() {
				rulerRequest(t, http.MethodDelete, "drift", "")
			}),
			driftRepairStep(func() {
				rulerRequest(t, http.MethodPost, "drift", testAccResourceNamespaceDriftEditedGroup)
			}),
			{
				PreConfig: func() {
					rulerRequest(t, http.MethodPost, "drift", testAccResourceNamespaceDriftExtraGroup)
				},
				Config: testAccResourceNamespaceDrift,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				Config: testAccResourceNamespaceDriftOneGroup,
				ConfigStateChecks: []statecheck.StateCheck{
					SemanticYAMLStateCheck("mimirtool_ruler_namespace.drift", "remote_config_yaml", testAccResourceNamespaceYaml),
				},
			},
		},
	})
}

const testAccResourceNamespaceDrift = `
provider "mimirtool" {
  address = "http://localhost:8080"
}

resource "mimirtool_ruler_namespace" "drift" {
	namespace = "drift"
	config_yaml = file("testdata/rules2.yaml")
  }
`

const testAccResourceNamespaceDriftOneGroup = `
provider "mimirtool" {
  address = "http://localhost:8080"
}

resource "mimirtool_ruler_namespace" "drift" {
	namespace = "drift"
	config_yaml = file("testdata/rules.yaml")
  }
`

const testAccResourceNamespaceDriftExpected = `groups:
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

const testAccResourceNamespaceDriftEditedGroup = `name: mimir_api_2
rules:
  - record: cluster_job_route:cortex_request_duration_seconds:99quantile
    expr: histogram_quantile(0.5, sum by (le, cluster, job, route) (rate(cortex_request_duration_seconds_bucket[1m])))
`

const testAccResourceNamespaceDriftExtraGroup = `name: out_of_band
rules:
  - record: job:up:sum
    expr: sum by (job) (up)
`

func TestAccResourceNamespaceDriftAllFields(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceNamespaceAllFields,
			},
			{
				PreConfig: func() {
					rulerRequest(t, http.MethodDelete, "all_fields/all_fields_records", "")
				},
				Config: testAccResourceNamespaceAllFields,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("mimirtool_ruler_namespace.all_fields", plancheck.ResourceActionUpdate),
					},
				},
			},
		},
	})
}

const testAccResourceNamespaceAllFields = `
provider "mimirtool" {
  address = "http://localhost:8080"
}

resource "mimirtool_ruler_namespace" "all_fields" {
	namespace = "all_fields"
	config_yaml = file("testdata/rules-all-fields.yaml")
	recording_rule_check = false
  }
`

// A state written by the released provider must plan nothing once upgraded,
// including for fields Mimir drops on write.
func TestAccResourceNamespaceUpgradeFromRelease(t *testing.T) {
	for _, fixture := range []string{"rules2.yaml", "rules-all-fields.yaml"} {
		t.Run(fixture, func(t *testing.T) {
			config := fmt.Sprintf(testAccResourceNamespaceUpgrade, fixture)
			resource.Test(t, resource.TestCase{
				Steps: []resource.TestStep{
					{
						ExternalProviders: map[string]resource.ExternalProvider{
							"mimirtool": {Source: "ovh/mimirtool", VersionConstraint: "1.0.0"},
						},
						Config: config,
					},
					{
						ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
						Config:                   config,
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PreApply: []plancheck.PlanCheck{
								plancheck.ExpectEmptyPlan(),
							},
						},
					},
				},
			})
		})
	}
}

const testAccResourceNamespaceUpgrade = `
provider "mimirtool" {
  address = "http://localhost:8080"
}

resource "mimirtool_ruler_namespace" "upgrade" {
	namespace = "upgrade"
	config_yaml = file("testdata/%s")
	recording_rule_check = false
  }
`

func TestNamespaceDrifted(t *testing.T) {
	const config = `groups:
  - name: a
    rules:
      - record: job:up:sum
        expr: sum(up) by (job)
`
	const applied = `groups:
    - name: a
      rules:
        - record: job:up:sum
          expr: sum by (job) (up)
`
	cases := []struct {
		name    string
		current string
		want    bool
	}{
		{"unchanged", applied, false},
		{"same rules, other formatting", "groups:\n- name: a\n  rules:\n  - expr: sum(up) by (job)\n    record: \"job:up:sum\"\n", false},
		{"group deleted", "groups: []\n", true},
		{"rule edited", "groups:\n  - name: a\n    rules:\n      - record: job:up:sum\n        expr: count(up)\n", true},
		{"extra group only in Mimir", applied + "    - name: b\n      rules:\n        - record: x:y:z\n          expr: up\n", false},
		{"unparsable group before a configured one", "groups:\n  - name: 0\n    rules:\n      - record: x:y:z\n        expr: 'sum('\n  - name: a\n    rules:\n      - record: job:up:sum\n        expr: sum(up) by (job)\n", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := namespaceDrifted(config, applied, c.current)
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Fatalf("namespaceDrifted() = %v, want %v", got, c.want)
			}
		})
	}
}
