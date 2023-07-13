package grafana_test

import (
	"testing"

	gapi "github.com/grafana/grafana-api-golang-client"
	"github.com/grafana/terraform-provider-grafana/internal/testutils"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccDatasourceRole(t *testing.T) {
	testutils.CheckEnterpriseTestsEnabled(t)

	var role gapi.Role
	checks := []resource.TestCheckFunc{
		testAccRoleCheckExists("grafana_role.test", &role),
		resource.TestCheckResourceAttr("data.grafana_role.from_name", "name", "test-role"),
		resource.TestCheckResourceAttr("data.grafana_role.from_name", "description", "test-role description"),
		resource.TestCheckResourceAttr("data.grafana_role.from_name", "uid", "test-ds-role-uid"),
		resource.TestCheckResourceAttr("data.grafana_role.from_name", "version", "1"),
		resource.TestCheckResourceAttr("data.grafana_role.from_name", "global", "true"),
		resource.TestCheckResourceAttr("data.grafana_role.from_name", "permissions.#", "3"),
		resource.TestCheckResourceAttr("data.grafana_role.from_name", "permissions.0.action", "org.users:add"),
		resource.TestCheckResourceAttr("data.grafana_role.from_name", "permissions.0.scope", "users:*"),
		resource.TestCheckResourceAttr("data.grafana_role.from_name", "permissions.1.action", "org.users:read"),
		resource.TestCheckResourceAttr("data.grafana_role.from_name", "permissions.1.scope", "users:*"),
		resource.TestCheckResourceAttr("data.grafana_role.from_name", "permissions.2.action", "org.users:write"),
		resource.TestCheckResourceAttr("data.grafana_role.from_name", "permissions.2.scope", "users:*"),
	}

	resource.ParallelTest(t, resource.TestCase{
		ProviderFactories: testutils.ProviderFactories,
		CheckDestroy:      testAccRoleCheckDestroy(&role),
		Steps: []resource.TestStep{
			{
				Config: testutils.TestAccExample(t, "data-sources/grafana_role/data-source.tf"),
				Check:  resource.ComposeTestCheckFunc(checks...),
			},
		},
	})
}