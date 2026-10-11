## Unreleased

BREAKING CHANGES:

* resource/mimirtool_ruler_namespace: Each apply now removes the rule groups of the namespace that are not in `config_yaml`, on create too. Before, the first apply kept them. If another tool or another resource block manages groups in the same namespace, move them to their own namespace before you upgrade.
* resource/mimirtool_ruler_namespace: `remote_config_yaml` is read-only. If your configuration sets it, remove it. Before, the apply failed with "Provider produced inconsistent result". Now the plan fails.
* provider: Unknown values in the provider configuration now cause an error. Before, the provider used the environment variables or the defaults instead, and could target another address or tenant.

BUG FIXES:

* resource/mimirtool_alertmanager: A configuration without `templates_config_yaml` no longer shows an update on every plan.
* resource/mimirtool_ruler_namespace: Delete matches the not-found error by type, not by error text, and its error names the namespace.
* resource/mimirtool_ruler_namespace: A namespace name with a space, `/`, `?` or another character that URLs escape now works. Before, the apply wrote the rule groups to Mimir and then failed to read them back.
* resource/mimirtool_ruler_namespace: Import fails when the namespace does not exist in Mimir for the configured tenant. Before, the import succeeded with no rule group.

## 0.1.0 (November 15, 2022)

Initial version
