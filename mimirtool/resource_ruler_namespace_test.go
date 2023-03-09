package mimirtool

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/grafana/mimir/pkg/mimirtool/rules/rwrulefmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/prometheus/prometheus/model/rulefmt"
	"gopkg.in/yaml.v3"
)

func initMockNamespace(t *testing.T) *MockMimirClientInterface {
	namespaceName := "demo"
	ctrl := gomock.NewController(t)
	mock := &MockMimirClientInterface{ctrl: ctrl}
	mock.recorder = &MockMimirClientInterfaceMockRecorder{mock}

	mock.EXPECT().CreateRuleGroup(gomock.Any(), namespaceName, gomock.Any()).AnyTimes().Return(nil)

	gomock.InOrder(
		mock.EXPECT().ListRules(gomock.Any(), namespaceName).MaxTimes(3).DoAndReturn(func(ctx context.Context, namespace string) (map[string][]rwrulefmt.RuleGroup, error) {
			return map[string][]rwrulefmt.RuleGroup{
				namespaceName: {
					{
						RuleGroup: rulefmt.RuleGroup{
							Name: "mimir_api_1",
							Rules: []rulefmt.RuleNode{
								{Record: yaml.Node{Value: "cluster_job:cortex_request_duration_seconds:99quantile", Kind: yaml.ScalarNode},
									Expr: yaml.Node{Value: "histogram_quantile(0.99, sum(rate(cortex_request_duration_seconds_bucket[1m])) by (le, cluster, job))", Kind: yaml.ScalarNode}},
								{Record: yaml.Node{Value: "cluster_job:cortex_request_duration_seconds:50quantile", Kind: yaml.ScalarNode},
									Expr: yaml.Node{Value: "histogram_quantile(0.50, sum(rate(cortex_request_duration_seconds_bucket[1m])) by (le, cluster, job))", Kind: yaml.ScalarNode}},
							},
						},
					},
				},
			}, nil
		}),
		// Doing the update in place
		mock.EXPECT().ListRules(gomock.Any(), namespaceName).MaxTimes(3).DoAndReturn(func(ctx context.Context, namespace string) (map[string][]rwrulefmt.RuleGroup, error) {
			return map[string][]rwrulefmt.RuleGroup{
				namespaceName: {
					{
						RuleGroup: rulefmt.RuleGroup{
							Name: "mimir_api_1",
							Rules: []rulefmt.RuleNode{
								{Record: yaml.Node{Value: "cluster_job:cortex_request_duration_seconds:99quantile", Kind: yaml.ScalarNode},
									Expr: yaml.Node{Value: "histogram_quantile(0.99, sum(rate(cortex_request_duration_seconds_bucket[1m])) by (le, cluster, job))", Kind: yaml.ScalarNode}},
								{Record: yaml.Node{Value: "cluster_job:cortex_request_duration_seconds:50quantile", Kind: yaml.ScalarNode},
									Expr: yaml.Node{Value: "histogram_quantile(0.50, sum(rate(cortex_request_duration_seconds_bucket[1m])) by (le, cluster, job))", Kind: yaml.ScalarNode}},
							},
						},
					},
					{
						RuleGroup: rulefmt.RuleGroup{
							Name: "mimir_api_2",
							Rules: []rulefmt.RuleNode{
								{Record: yaml.Node{Value: "cluster_job_route:cortex_request_duration_seconds:99quantile", Kind: yaml.ScalarNode},
									Expr: yaml.Node{Value: "histogram_quantile(0.99, sum(rate(cortex_request_duration_seconds_bucket[1m])) by (le, cluster, job, route))", Kind: yaml.ScalarNode}},
							},
						},
					},
				},
			}, nil
		}),
	)

	mock.EXPECT().DeleteNamespace(gomock.Any(), gomock.Any()).Return(nil)
	return mock
}

func TestAccResourceNamespace(t *testing.T) {
	for _, useSHA256 := range []bool{false, true} {
		os.Setenv("MIMIR_STORE_RULES_SHA256", fmt.Sprintf("%t", useSHA256))
		defer os.Unsetenv("MIMIR_STORE_RULES_SHA256")

		expectedInitialConfig := testAccResourceNamespaceYaml
		expectedInitialConfigAfterUpdate := testAccResourceNamespaceYamlAfterUpdate
		if useSHA256 {
			expectedInitialConfig = "1f42376cda18887c0611a56aa432f8897dac5a3e9a94b839486aa1c8a0c94375"
			expectedInitialConfigAfterUpdate = "5eb3566fb3eabf583b2301c63b3629ee4003a7443360a8015bd15da3cd17cad6"
		}
		mockClient = initMockNamespace(t)
		defer mockClient.ctrl.Finish()
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
	mockClient = initMockNamespace(t)
	defer mockClient.ctrl.Finish()
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
						"mimirtool_ruler_namespace.demo", "config_yaml", testAccResourceNamespaceYaml),
				),
			},
		},
	})

}

func TestAccResourceNamespaceCheckRules(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := &MockMimirClientInterface{ctrl: ctrl}
	mock.recorder = &MockMimirClientInterfaceMockRecorder{mock}
	defer mock.ctrl.Finish()
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
	ctrl := gomock.NewController(t)
	mock := &MockMimirClientInterface{ctrl: ctrl}
	mock.recorder = &MockMimirClientInterfaceMockRecorder{mock}
	defer mock.ctrl.Finish()
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
