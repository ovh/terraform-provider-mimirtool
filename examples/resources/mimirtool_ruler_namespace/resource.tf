resource "mimirtool_ruler_namespace" "demo" {
  namespace   = "demo"
  config_yaml = <<EOT
groups:
- name: mimir_api_1
  rules:
  - expr: histogram_quantile(0.99, sum(rate(cortex_request_duration_seconds_bucket[1m]))
      by (le, cluster, job))
    record: cluster_job:cortex_request_duration_seconds:99quantile
  - expr: histogram_quantile(0.50, sum(rate(cortex_request_duration_seconds_bucket[1m]))
      by (le, cluster, job))
    record: cluster_job:cortex_request_duration_seconds:50quantile
EOT
}
