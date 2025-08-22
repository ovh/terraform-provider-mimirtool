// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"os"

	"github.com/grafana/dskit/crypto/tls"
	mimirtool "github.com/grafana/mimir/pkg/mimirtool/client"
	mimirVersion "github.com/grafana/mimir/pkg/util/version"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure MimirtoolProvider satisfies various provider interfaces.
var _ provider.Provider = &MimirtoolProvider{}

// MimirtoolProvider defines the provider implementation.
type MimirtoolProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// MimirClientConfig holds the configuration for the Mimir client
type MimirClientConfig struct {
	Address                string
	TenantID               string
	APIUser                string
	APIKey                 string
	AuthToken              string
	TLSKeyPath             string
	TLSCertPath            string
	TLSCAPath              string
	InsecureSkipVerify     bool
	PrometheusHTTPPrefix   string
	AlertmanagerHTTPPrefix string
}

// MimirtoolProviderModel describes the provider data model.
type MimirtoolProviderModel struct {
	Address                types.String `tfsdk:"address"`
	TenantID               types.String `tfsdk:"tenant_id"`
	APIUser                types.String `tfsdk:"api_user"`
	APIKey                 types.String `tfsdk:"api_key"`
	AuthToken              types.String `tfsdk:"auth_token"`
	TLSKeyPath             types.String `tfsdk:"tls_key_path"`
	TLSCertPath            types.String `tfsdk:"tls_cert_path"`
	TLSCAPath              types.String `tfsdk:"tls_ca_path"`
	InsecureSkipVerify     types.Bool   `tfsdk:"insecure_skip_verify"`
	PrometheusHTTPPrefix   types.String `tfsdk:"prometheus_http_prefix"`
	AlertmanagerHTTPPrefix types.String `tfsdk:"alertmanager_http_prefix"`
}

func (p *MimirtoolProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "mimirtool"
	resp.Version = p.version
}

func (p *MimirtoolProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The Mimirtool provider allows you to manage Grafana Mimir resources using Terraform.",
		Attributes: map[string]schema.Attribute{
			"address": schema.StringAttribute{
				MarkdownDescription: "Address to use when contacting Grafana Mimir. May alternatively be set via the `MIMIRTOOL_ADDRESS` or `MIMIR_ADDRESS` environment variable.",
				Optional:            true,
			},
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "Tenant ID to use when contacting Grafana Mimir. May alternatively be set via the `MIMIRTOOL_TENANT_ID` or `MIMIR_TENANT_ID` environment variable.",
				Optional:            true,
			},
			"api_user": schema.StringAttribute{
				MarkdownDescription: "API user to use when contacting Grafana Mimir. May alternatively be set via the `MIMIRTOOL_API_USER` or `MIMIR_API_USER` environment variable.",
				Optional:            true,
			},
			"api_key": schema.StringAttribute{
				MarkdownDescription: "API key to use when contacting Grafana Mimir. May alternatively be set via the `MIMIRTOOL_API_KEY` or `MIMIR_API_KEY` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"auth_token": schema.StringAttribute{
				MarkdownDescription: "Authentication token for bearer token or JWT auth when contacting Grafana Mimir. May alternatively be set via the `MIMIRTOOL_AUTH_TOKEN` or `MIMIR_AUTH_TOKEN` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"tls_key_path": schema.StringAttribute{
				MarkdownDescription: "Client TLS key file to use to authenticate to the MIMIR server. May alternatively be set via the `MIMIRTOOL_TLS_KEY_PATH` or `MIMIR_TLS_KEY_PATH` environment variable.",
				Optional:            true,
			},
			"tls_cert_path": schema.StringAttribute{
				MarkdownDescription: "Client TLS certificate file to use to authenticate to the MIMIR server. May alternatively be set via the `MIMIRTOOL_TLS_CERT_PATH` or `MIMIR_TLS_CERT_PATH` environment variable.",
				Optional:            true,
			},
			"tls_ca_path": schema.StringAttribute{
				MarkdownDescription: "Certificate CA bundle to use to verify the MIMIR server's certificate. May alternatively be set via the `MIMIRTOOL_TLS_CA_PATH` or `MIMIR_TLS_CA_PATH` environment variable.",
				Optional:            true,
			},
			"insecure_skip_verify": schema.BoolAttribute{
				MarkdownDescription: "Skip TLS certificate verification. May alternatively be set via the `MIMIRTOOL_INSECURE_SKIP_VERIFY` or `MIMIR_INSECURE_SKIP_VERIFY` environment variable.",
				Optional:            true,
			},
			"prometheus_http_prefix": schema.StringAttribute{
				MarkdownDescription: "Path prefix to use for rules. May alternatively be set via the `MIMIRTOOL_PROMETHEUS_HTTP_PREFIX` or `MIMIR_PROMETHEUS_HTTP_PREFIX` environment variable.",
				Optional:            true,
			},
			"alertmanager_http_prefix": schema.StringAttribute{
				MarkdownDescription: "Path prefix to use for alertmanager. May alternatively be set via the `MIMIRTOOL_ALERTMANAGER_HTTP_PREFIX` or `MIMIR_ALERTMANAGER_HTTP_PREFIX` environment variable.",
				Optional:            true,
			},
		},
	}
}

func (p *MimirtoolProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data MimirtoolProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Get values from config or environment variables
	clientConfig := MimirClientConfig{
		Address:                getStringValue(data.Address, "MIMIRTOOL_ADDRESS", "MIMIR_ADDRESS", ""),
		TenantID:               getStringValue(data.TenantID, "MIMIRTOOL_TENANT_ID", "MIMIR_TENANT_ID", ""),
		APIUser:                getStringValue(data.APIUser, "MIMIRTOOL_API_USER", "MIMIR_API_USER", ""),
		APIKey:                 getStringValue(data.APIKey, "MIMIRTOOL_API_KEY", "MIMIR_API_KEY", ""),
		AuthToken:              getStringValue(data.AuthToken, "MIMIRTOOL_AUTH_TOKEN", "MIMIR_AUTH_TOKEN", ""),
		TLSKeyPath:             getStringValue(data.TLSKeyPath, "MIMIRTOOL_TLS_KEY_PATH", "MIMIR_TLS_KEY_PATH", ""),
		TLSCertPath:            getStringValue(data.TLSCertPath, "MIMIRTOOL_TLS_CERT_PATH", "MIMIR_TLS_CERT_PATH", ""),
		TLSCAPath:              getStringValue(data.TLSCAPath, "MIMIRTOOL_TLS_CA_PATH", "MIMIR_TLS_CA_PATH", ""),
		InsecureSkipVerify:     getBoolValue(data.InsecureSkipVerify, "MIMIRTOOL_INSECURE_SKIP_VERIFY", "MIMIR_INSECURE_SKIP_VERIFY", false),
		PrometheusHTTPPrefix:   getStringValue(data.PrometheusHTTPPrefix, "MIMIRTOOL_PROMETHEUS_HTTP_PREFIX", "MIMIR_PROMETHEUS_HTTP_PREFIX", "/prometheus"),
		AlertmanagerHTTPPrefix: getStringValue(data.AlertmanagerHTTPPrefix, "MIMIRTOOL_ALERTMANAGER_HTTP_PREFIX", "MIMIR_ALERTMANAGER_HTTP_PREFIX", "/alertmanager"),
	}

	tflog.Info(ctx, "Configured Mimirtool provider", map[string]interface{}{
		"address":                  clientConfig.Address,
		"tenant_id":                clientConfig.TenantID,
		"prometheus_http_prefix":   clientConfig.PrometheusHTTPPrefix,
		"alertmanager_http_prefix": clientConfig.AlertmanagerHTTPPrefix,
	})

	// Validate required fields
	if clientConfig.Address == "" {
		resp.Diagnostics.AddError(
			"Missing Required Configuration",
			"The provider cannot create the Mimir client as there is a missing or empty value for the \"address\" configuration. "+
				"Set the address value in the configuration or use the MIMIRTOOL_ADDRESS or MIMIR_ADDRESS environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
		return
	}

	// Create a new Mimirtool client using the configuration values
	var err error
	c := &myClient{}
	c.cli, err = getDefaultMimirClient(clientConfig, p.version)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Mimirtool API Client",
			"An unexpected error occurred when creating the Mimirtool API client. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"Mimirtool Client Error: "+err.Error(),
		)
		return
	}

	resp.DataSourceData = c.cli
	resp.ResourceData = c.cli
}

func getDefaultMimirClient(cfg MimirClientConfig, version string) (mimirClientInterface, error) {
	mimirVersion.Version = fmt.Sprintf("terraform-provider-mimirtool-%s", version)
	return mimirtool.New(mimirtool.Config{
		AuthToken: cfg.AuthToken,
		User:      cfg.APIUser,
		Key:       cfg.APIKey,
		Address:   cfg.Address,
		ID:        cfg.TenantID,
		TLS: tls.ClientConfig{
			CAPath:             cfg.TLSCAPath,
			CertPath:           cfg.TLSCertPath,
			KeyPath:            cfg.TLSKeyPath,
			InsecureSkipVerify: cfg.InsecureSkipVerify,
		},
	})
}

func (p *MimirtoolProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewRulerNamespaceResource,
		NewAlertmanagerResource,
	}
}

func (p *MimirtoolProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &MimirtoolProvider{
			version: version,
		}
	}
}

// Helper functions to get values from config or environment variables
func getStringValue(configValue types.String, envVar1, envVar2, defaultValue string) string {
	if !configValue.IsNull() && !configValue.IsUnknown() {
		return configValue.ValueString()
	}

	// Try first environment variable
	if value := os.Getenv(envVar1); value != "" {
		return value
	}

	// Try second environment variable
	if value := os.Getenv(envVar2); value != "" {
		return value
	}

	return defaultValue
}

func getBoolValue(configValue types.Bool, envVar1, envVar2 string, defaultValue bool) bool {
	if !configValue.IsNull() && !configValue.IsUnknown() {
		return configValue.ValueBool()
	}

	// Try first environment variable
	if value := os.Getenv(envVar1); value != "" {
		return value == "true" || value == "1"
	}

	// Try second environment variable
	if value := os.Getenv(envVar2); value != "" {
		return value == "true" || value == "1"
	}

	return defaultValue
}
