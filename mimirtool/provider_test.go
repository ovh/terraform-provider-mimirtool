package mimirtool

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func getSetEnv(key, fallback string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		value = fallback
		os.Setenv(key, fallback)
	}
	return value
}

// testAccProviderFactories is a static map containing only the main provider instance
var testAccProviderFactories map[string]func() (*schema.Provider, error)

// testAccProvider is the "main" provider instance
//
// This Provider can be used in testing code for API calls without requiring
// the use of saving and referencing specific ProviderFactories instances.
//
// testAccPreCheck(t) must be called before using this provider instance.
var testAccProvider *schema.Provider

var testAccProviders map[string]*schema.Provider

// testAccProviderConfigure ensures testAccProvider is only configured once
//
// The testAccPreCheck(t) function is invoked for every test and this prevents
// extraneous reconfiguration to the same values each time. However, this does
// not prevent reconfiguration that may happen should the address of
// testAccProvider be errantly reused in ProviderFactories.
var testAccProviderConfigure sync.Once

func init() {
	testAccProvider = New("dev")()
	testAccProviders = map[string]*schema.Provider{
		"mimirtool": New("dev")(),
	}

	// Always allocate a new provider instance each invocation, otherwise gRPC
	// ProviderConfigure() can overwrite configuration during concurrent testing.
	testAccProviderFactories = map[string]func() (*schema.Provider, error){
		"mimirtool": func() (*schema.Provider, error) {
			return New("dev")(), nil
		},
	}
}

func TestProvider(t *testing.T) {
	if err := New("dev")().InternalValidate(); err != nil {
		t.Fatalf("err: %s", err)
	}
}

// testAccPreCheck verifies required provider testing configuration. It should
// be present in every acceptance test.
//
// These verifications and configuration are preferred at this level to prevent
// provider developers from experiencing less clear errors for every test.
func testAccPreCheck(t *testing.T) {
	testAccProviderConfigure.Do(func() {
		// Since we are outside the scope of the Terraform configuration we must
		// call Configure() to properly initialize the provider configuration.
		err := testAccProvider.Configure(context.Background(), terraform.NewResourceConfigRaw(nil))
		if err != nil {
			t.Fatal(err)
		}
	})
}
