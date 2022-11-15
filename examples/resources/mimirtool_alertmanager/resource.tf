resource "mimirtool_alertmanager" "demo" {
  config_yaml = file("testdata/example_alertmanager_config.yaml")
  templates_config_yaml = {
    default_template = file("testdata/example_alertmanager_template.tmpl")
  }
}
