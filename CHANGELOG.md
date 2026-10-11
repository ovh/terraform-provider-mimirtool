## Unreleased

BREAKING CHANGES:

* resource/mimirtool_ruler_namespace: `remote_config_yaml` is read-only. If your configuration sets it, remove it. Before, the apply failed with "Provider produced inconsistent result". Now the plan fails.

BUG FIXES:

* resource/mimirtool_alertmanager: A configuration without `templates_config_yaml` no longer shows an update on every plan.

## 0.1.0 (November 15, 2022)

Initial version
