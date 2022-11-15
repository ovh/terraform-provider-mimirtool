package mimirtool

import (
	context "context"

	rwrulefmt "github.com/grafana/mimir/pkg/mimirtool/rules/rwrulefmt"
)

type client struct {
	cli mimirClientInterface
}

type mimirClientInterface interface {
	// Ruler
	DeleteRuleGroup(ctx context.Context, namespace string, groupName string) error
	ListRules(ctx context.Context, namespace string) (map[string][]rwrulefmt.RuleGroup, error)
	DeleteNamespace(ctx context.Context, namespace string) error
	CreateRuleGroup(ctx context.Context, namespace string, rg rwrulefmt.RuleGroup) error
	// Alertmanager
	CreateAlertmanagerConfig(ctx context.Context, cfg string, templates map[string]string) error
	GetAlertmanagerConfig(ctx context.Context) (string, map[string]string, error)
	DeleteAlermanagerConfig(ctx context.Context) error
}
