package github

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccGithubOrganizationImmutableReleases(t *testing.T) {
	t.Run("test setting enforced_repositories to all", func(t *testing.T) {
		config := `
			resource "github_organization_immutable_releases" "test" {
				enforced_repositories = "all"
			}
		`

		check := resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr(
				"github_organization_immutable_releases.test", "enforced_repositories", "all",
			),
		)

		resource.Test(t, resource.TestCase{
			PreCheck:          func() { skipUnlessHasOrgs(t) },
			ProviderFactories: providerFactories,
			CheckDestroy:      testAccCheckGithubOrganizationImmutableReleasesDestroy,
			Steps: []resource.TestStep{
				{
					Config: config,
					Check:  check,
				},
			},
		})
	})

	t.Run("test setting enforced_repositories to none", func(t *testing.T) {
		config := `
			resource "github_organization_immutable_releases" "test" {
				enforced_repositories = "none"
			}
		`

		check := resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr(
				"github_organization_immutable_releases.test", "enforced_repositories", "none",
			),
		)

		resource.Test(t, resource.TestCase{
			PreCheck:          func() { skipUnlessHasOrgs(t) },
			ProviderFactories: providerFactories,
			CheckDestroy:      testAccCheckGithubOrganizationImmutableReleasesDestroy,
			Steps: []resource.TestStep{
				{
					Config: config,
					Check:  check,
				},
			},
		})
	})

	t.Run("test setting enforced_repositories to selected with repository IDs", func(t *testing.T) {
		randomID := acctest.RandStringFromCharSet(5, acctest.CharSetAlphaNum)
		repoName := fmt.Sprintf("%srepo-immutable-rel-%s", testResourcePrefix, randomID)

		config := fmt.Sprintf(`
			resource "github_repository" "test" {
				name        = "%[1]s"
				description = "Terraform acceptance tests %[1]s"
				topics      = ["terraform", "testing"]
			}

			resource "github_organization_immutable_releases" "test" {
				enforced_repositories  = "selected"
				selected_repository_ids = [github_repository.test.repo_id]
			}
		`, repoName)

		check := resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr(
				"github_organization_immutable_releases.test", "enforced_repositories", "selected",
			),
			resource.TestCheckResourceAttr(
				"github_organization_immutable_releases.test", "selected_repository_ids.#", "1",
			),
		)

		resource.Test(t, resource.TestCase{
			PreCheck:          func() { skipUnlessHasOrgs(t) },
			ProviderFactories: providerFactories,
			CheckDestroy:      testAccCheckGithubOrganizationImmutableReleasesDestroy,
			Steps: []resource.TestStep{
				{
					Config: config,
					Check:  check,
				},
			},
		})
	})

	t.Run("test import of organization immutable releases", func(t *testing.T) {
		config := `
			resource "github_organization_immutable_releases" "test" {
				enforced_repositories = "all"
			}
		`

		check := resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr(
				"github_organization_immutable_releases.test", "enforced_repositories", "all",
			),
		)

		resource.Test(t, resource.TestCase{
			PreCheck:          func() { skipUnlessHasOrgs(t) },
			ProviderFactories: providerFactories,
			CheckDestroy:      testAccCheckGithubOrganizationImmutableReleasesDestroy,
			Steps: []resource.TestStep{
				{
					Config: config,
					Check:  check,
				},
				{
					ResourceName:      "github_organization_immutable_releases.test",
					ImportState:       true,
					ImportStateVerify: true,
				},
			},
		})
	})

	t.Run("test updating enforced_repositories from all to selected", func(t *testing.T) {
		randomID := acctest.RandStringFromCharSet(5, acctest.CharSetAlphaNum)
		repoName := fmt.Sprintf("%srepo-immutable-rel-%s", testResourcePrefix, randomID)

		configAll := `
			resource "github_organization_immutable_releases" "test" {
				enforced_repositories = "all"
			}
		`

		configSelected := fmt.Sprintf(`
			resource "github_repository" "test" {
				name        = "%[1]s"
				description = "Terraform acceptance tests %[1]s"
				topics      = ["terraform", "testing"]
			}

			resource "github_organization_immutable_releases" "test" {
				enforced_repositories  = "selected"
				selected_repository_ids = [github_repository.test.repo_id]
			}
		`, repoName)

		checkAll := resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr(
				"github_organization_immutable_releases.test", "enforced_repositories", "all",
			),
		)

		checkSelected := resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr(
				"github_organization_immutable_releases.test", "enforced_repositories", "selected",
			),
			resource.TestCheckResourceAttr(
				"github_organization_immutable_releases.test", "selected_repository_ids.#", "1",
			),
		)

		resource.Test(t, resource.TestCase{
			PreCheck:          func() { skipUnlessHasOrgs(t) },
			ProviderFactories: providerFactories,
			CheckDestroy:      testAccCheckGithubOrganizationImmutableReleasesDestroy,
			Steps: []resource.TestStep{
				{
					Config: configAll,
					Check:  checkAll,
				},
				{
					Config: configSelected,
					Check:  checkSelected,
				},
			},
		})
	})
}

func testAccCheckGithubOrganizationImmutableReleasesDestroy(s *terraform.State) error {
	meta, err := getTestMeta()
	if err != nil {
		return err
	}
	conn := meta.v3client
	orgName := meta.name

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "github_organization_immutable_releases" {
			continue
		}

		settings, resp, err := conn.Organizations.GetImmutableReleasesSettings(context.Background(), orgName)
		if err != nil {
			if resp != nil && resp.StatusCode == 404 {
				continue
			}
			return err
		}
		if settings.GetEnforcedRepositories() != "none" {
			return fmt.Errorf("immutable releases still enforced for organization %s: %s", orgName, settings.GetEnforcedRepositories())
		}
	}

	return nil
}
