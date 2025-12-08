package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// testAccProtoV6ProviderFactories are used to instantiate a provider during
// acceptance testing. The factory function will be invoked for every Terraform
// CLI command executed to create a provider server to which the CLI can
// reattach.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"hostinger": providerserver.NewProtocol6WithError(New("test")()),
}

func TestAccWebsiteResource(t *testing.T) {
	// Retrieve Order ID from env var or use default for testing
	orderID := os.Getenv("HOSTINGER_ORDER_ID")
	if orderID == "" {
		orderID = "1006933104" // Default test order ID
	}

	domain := "test-acc-terraform-" + os.Getenv("USER") + ".com"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccWebsiteResourceConfig(domain, orderID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hostinger_website.test", "domain", domain),
					resource.TestCheckResourceAttr("hostinger_website.test", "order_id", orderID),
					resource.TestCheckResourceAttrSet("hostinger_website.test", "id"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "hostinger_website.test",
				ImportState:       true,
				ImportStateVerify: true,
				// The ID is the domain, but some attributes like datacenter_code 
				// might not be returned by API read, so we might need to ignore them if they differ.
				// However, for basic fields it should match.
			},
			// Update testing (not supported, should recreate)
			// We skip this for now as Hostinger API doesn't support update
		},
	})
}

func testAccPreCheck(t *testing.T) {
	if v := os.Getenv("HOSTINGER_API_TOKEN"); v == "" {
		t.Fatal("HOSTINGER_API_TOKEN must be set for acceptance tests")
	}
}

func testAccWebsiteResourceConfig(domain, orderID string) string {
	return fmt.Sprintf(`
provider "hostinger" {}

resource "hostinger_website" "test" {
  domain   = %[1]q
  order_id = %[2]s
}
`, domain, orderID)
}
