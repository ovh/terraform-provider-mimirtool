package mimirtool

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func initMockAlertmanager(t *testing.T) *MockMimirClientInterface {
	ctrl := gomock.NewController(t)
	mock := &MockMimirClientInterface{ctrl: ctrl}
	mock.recorder = &MockMimirClientInterfaceMockRecorder{mock}

	mock.EXPECT().CreateAlertmanagerConfig(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(nil)
	mock.EXPECT().DeleteAlermanagerConfig(gomock.Any()).AnyTimes().Return(nil)
	osef := map[string]string{
		"default_template": testAccResourceAlertmanagerTemplate,
	}
	mock.EXPECT().GetAlertmanagerConfig(gomock.Any()).AnyTimes().Return(testAccResourceAlertmanagerYaml, osef, nil)
	return mock
}

func TestAccResourceAlertmanager(t *testing.T) {
	mockClient = initMockAlertmanager(t)
	defer mockClient.ctrl.Finish()
	resource.UnitTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceAlertmanager,
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

const testAccResourceAlertmanager = `
resource "mimirtool_alertmanager" "demo" {
	config_yaml = file("testdata/example_alertmanager_config.yaml")
	templates_config_yaml = {
	  default_template = file("testdata/example_alertmanager_template.tmpl")
	}
  }
`

const testAccResourceAlertmanagerYaml = `---
# See: https://grafana.com/docs/mimir/latest/operators-guide/reference-http-api/#alertmanager
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
