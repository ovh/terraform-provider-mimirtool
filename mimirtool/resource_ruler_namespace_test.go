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
			expectedInitialConfig = "8ee2cdb65b41d7bdc875ebec6cb1f7a9ab0813e9f6166105a7953d7bf9a68d9b"
			expectedInitialConfigAfterUpdate = "372a400b1eae63b184c036dd2c0aaf71c57e3d8125621e371389d4f7c40c1cb6"
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
