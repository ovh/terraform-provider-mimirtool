resource "mimirtool_alertmanager" "demo" {
  config_yaml = <<EOT
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
EOT
  templates_config_yaml = {
    default_template = <<EOT
{{ define "__alertmanager" }}AlertManager{{ end }}
{{ define "__alertmanagerURL" }}{{ .ExternalURL }}/#/alerts?receiver={{ .Receiver | urlquery }}{{ end }}
EOT
  }
}
