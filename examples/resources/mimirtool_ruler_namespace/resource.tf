resource "mimirtool_ruler_namespace" "demo" {
  namespace   = "demo"
  config_yaml = file("testdata/rules.yaml")
}
