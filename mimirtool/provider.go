package mimirtool

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"

	"github.com/grafana/dskit/crypto/tls"
	mimirtool "github.com/grafana/mimir/pkg/mimirtool/client"
	mimirVersion "github.com/grafana/mimir/pkg/util/version"
)

func init() {
	// Set descriptions to support markdown syntax, this will be used in document generation
	// and the language server.
	schema.DescriptionKind = schema.StringMarkdown

	// Customize the content of descriptions when output. For example you can add defaults on
	// to the exported descriptions if present.
	// schema.SchemaDescriptionBuilder = func(s *schema.Schema) string {
	// 	desc := s.Description
	// 	if s.Default != nil {
	// 		desc += fmt.Sprintf(" Defaults to `%v`.", s.Default)
	// 	}
	// 	return strings.TrimSpace(desc)
	// }
}

// New returns a newly created provider
func New(version string) func() *schema.Provider {
	return func() *schema.Provider {
		p := &schema.Provider{
			Schema: map[string]*schema.Schema{
				// In order to allow users to use both terraform and mimirtool cli let's use the same envvar names
				// We shall accept two envvar name: one to respect terraform convention <provider>_<resource_name> and the other one from mimirtool.
				// terraform convention will be taken into account first.
				"address": {
					Type:         schema.TypeString,
					Required:     true,
					DefaultFunc:  schema.MultiEnvDefaultFunc([]string{"MIMIRTOOL_ADDRESS", "MIMIR_ADDRESS"}, nil),
					Description:  "Address to use when contacting Grafana Mimir. May alternatively be set via the `MIMIRTOOL_ADDRESS` or `MIMIR_ADDRESS` environment variable.",
					ValidateFunc: validation.IsURLWithHTTPorHTTPS,
				},
				"tenant_id": {
					Type:        schema.TypeString,
					Optional:    true,
					DefaultFunc: schema.MultiEnvDefaultFunc([]string{"MIMIRTOOL_TENANT_ID", "MIMIR_TENANT_ID"}, nil),
					Description: "Tenant ID to use when contacting Grafana Mimir. May alternatively be set via the `MIMIRTOOL_TENANT_ID` or `MIMIR_TENANT_ID` environment variable.",
				},
				"api_user": {
					Type:        schema.TypeString,
					Optional:    true,
					DefaultFunc: schema.MultiEnvDefaultFunc([]string{"MIMIRTOOL_API_USER", "MIMIR_API_USER"}, nil),
					Description: "API user to use when contacting Grafana Mimir. May alternatively be set via the `MIMIRTOOL_API_USER` or `MIMIR_API_USER` environment variable.",
				},
				"api_key": {
					Type:        schema.TypeString,
					Optional:    true,
					Sensitive:   true,
					DefaultFunc: schema.MultiEnvDefaultFunc([]string{"MIMIRTOOL_API_KEY", "MIMIR_API_KEY"}, nil),
					Description: "API key to use when contacting Grafana Mimir. May alternatively be set via the `MIMIRTOOL_API_KEY` or `MIMIR_API_KEY` environment variable.",
				},
				"auth_token": {
					Type:        schema.TypeString,
					Optional:    true,
					Sensitive:   true,
					DefaultFunc: schema.MultiEnvDefaultFunc([]string{"MIMIRTOOL_AUTH_TOKEN", "MIMIR_AUTH_TOKEN"}, nil),
					Description: "Authentication token for bearer token or JWT auth when contacting Grafana Mimir. May alternatively be set via the `MIMIRTOOL_AUTH_TOKEN` or `MIMIR_AUTH_TOKEN` environment variable.",
				},
				"tls_key_path": {
					Type:        schema.TypeString,
					Optional:    true,
					DefaultFunc: schema.MultiEnvDefaultFunc([]string{"MIMIRTOOL_TLS_KEY_PATH", "MIMIR_TLS_KEY_PATH"}, nil),
					Description: "Client TLS key file to use to authenticate to the MIMIR server. May alternatively be set via the `MIMIRTOOL_TLS_KEY_PATH` or `MIMIR_TLS_KEY_PATH` environment variable.",
				},
				"tls_cert_path": {
					Type:        schema.TypeString,
					Optional:    true,
					DefaultFunc: schema.MultiEnvDefaultFunc([]string{"MIMIRTOOL_TLS_CERT_PATH", "MIMIR_TLS_CERT_PATH"}, nil),
					Description: "Client TLS certificate file to use to authenticate to the MIMIR server. May alternatively be set via the `MIMIRTOOL_TLS_CERT_PATH` or `MIMIR_TLS_CERT_PATH` environment variable.",
				},
				"tls_ca_path": {
					Type:        schema.TypeString,
					Optional:    true,
					DefaultFunc: schema.MultiEnvDefaultFunc([]string{"MIMIRTOOL_TLS_CA_PATH", "MIMIR_TLS_CA_PATH"}, nil),
					Description: "Certificate CA bundle to use to verify the MIMIR server's certificate. May alternatively be set via the `MIMIRTOOL_TLS_CA_PATH` or `MIMIR_TLS_CA_PATH` environment variable.",
				},
				"insecure_skip_verify": {
					Type:        schema.TypeBool,
					Optional:    true,
					DefaultFunc: schema.MultiEnvDefaultFunc([]string{"MIMIRTOOL_INSECURE_SKIP_VERIFY", "MIMIR_INSECURE_SKIP_VERIFY"}, nil),
					Description: "Skip TLS certificate verification. May alternatively be set via the `MIMIRTOOL_INSECURE_SKIP_VERIFY` or `MIMIR_INSECURE_SKIP_VERIFY` environment variable.",
				},
				"prometheus_http_prefix": {
					Type:        schema.TypeString,
					Optional:    true,
					DefaultFunc: schema.MultiEnvDefaultFunc([]string{"MIMIRTOOL_PROMETHEUS_HTTP_PREFIX", "MIMIR_PROMETHEUS_HTTP_PREFIX"}, "/prometheus"),
					Description: "Path prefix to use for rules. May alternatively be set via the `MIMIRTOOL_PROMETHEUS_HTTP_PREFIX` or `MIMIR_PROMETHEUS_HTTP_PREFIX` environment variable.",
				},
				"alertmanager_http_prefix": {
					Type:        schema.TypeString,
					Optional:    true,
					DefaultFunc: schema.MultiEnvDefaultFunc([]string{"MIMIRTOOL_ALERTMANAGER_HTTP_PREFIX", "MIMIR_ALERTMANAGER_HTTP_PREFIX"}, "/alertmanager"),
					Description: "Path prefix to use for alertmanager. May alternatively be set via the `MIMIRTOOL_ALERTMANAGER_HTTP_PREFIX` or `MIMIR_ALERTMANAGER_HTTP_PREFIX` environment variable.",
				},
			},
			DataSourcesMap: map[string]*schema.Resource{},
			ResourcesMap: map[string]*schema.Resource{
				"mimirtool_ruler_namespace": resourceRulerNamespace(),
				"mimirtool_alertmanager":    resourceAlertManager(),
			},
		}

		p.ConfigureContextFunc = configure(version, p)

		return p
	}
}

func configure(version string, p *schema.Provider) func(context.Context, *schema.ResourceData) (interface{}, diag.Diagnostics) {
	return func(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
		var (
			diags diag.Diagnostics
			err   error
		)

		c := &client{}

		c.cli, err = getDefaultMimirClient(d, version)
		if err != nil {
			return nil, diag.FromErr(err)
		}
		return c, diags
	}
}

func getDefaultMimirClient(d *schema.ResourceData, version string) (mimirClientInterface, error) {
	mimirVersion.Version = fmt.Sprintf("terraform-provider-mimirtool-%s", version)
	return mimirtool.New(mimirtool.Config{
		AuthToken: d.Get("auth_token").(string),
		User:      d.Get("api_user").(string),
		Key:       d.Get("api_key").(string),
		Address:   d.Get("address").(string),
		ID:        d.Get("tenant_id").(string),
		TLS: tls.ClientConfig{
			CAPath:             d.Get("tls_ca_path").(string),
			CertPath:           d.Get("tls_cert_path").(string),
			KeyPath:            d.Get("tls_key_path").(string),
			InsecureSkipVerify: d.Get("insecure_skip_verify").(bool),
		},
	})
}
