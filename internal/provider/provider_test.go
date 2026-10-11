// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
)

// testAccProtoV6ProviderFactories is used to instantiate a provider during acceptance testing.
// The factory function is called for each Terraform CLI command to create a provider
// server that the CLI can connect to and interact with.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"mimirtool": providerserver.NewProtocol6WithError(New("test")()),
}

// Each test gets its own tenant, so tests share no namespace or Alertmanager
// config and do not depend on the order they run in.
func testAccTenant() string {
	return acctest.RandomWithPrefix("tf-acc")
}

func testAccConfig(tenant, config string) string {
	return fmt.Sprintf(`
provider "mimirtool" {
  address   = "http://localhost:8080"
  tenant_id = %q
}
`, tenant) + config
}

func testAccGet(t *testing.T, tenant, url string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Scope-OrgID", tenant)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, string(body)
}

func TestConfigureRejectsUnknownValues(t *testing.T) {
	t.Setenv("MIMIRTOOL_TENANT_ID", "from-env")
	ctx := context.Background()
	p := New("test")()

	var schemaResp provider.SchemaResponse
	p.Schema(ctx, provider.SchemaRequest{}, &schemaResp)
	objectType := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)

	values := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
	for name, attrType := range objectType.AttributeTypes {
		values[name] = tftypes.NewValue(attrType, nil)
	}
	values["address"] = tftypes.NewValue(tftypes.String, "http://localhost:8080")
	values["tenant_id"] = tftypes.NewValue(tftypes.String, tftypes.UnknownValue)

	req := provider.ConfigureRequest{Config: tfsdk.Config{
		Schema: schemaResp.Schema,
		Raw:    tftypes.NewValue(objectType, values),
	}}
	var resp provider.ConfigureResponse
	p.Configure(ctx, req, &resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("Configure() accepted an unknown tenant_id and fell back to the environment")
	}
	if got := resp.Diagnostics.Errors()[0].Summary(); !strings.Contains(got, "tenant_id") {
		t.Errorf("error summary = %q, want it to name tenant_id", got)
	}
}
