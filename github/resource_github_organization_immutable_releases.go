package github

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/google/go-github/v83/github"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/customdiff"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceGithubOrganizationImmutableReleases() *schema.Resource {
	return &schema.Resource{
		Create: resourceGithubOrganizationImmutableReleasesCreateOrUpdate,
		Read:   resourceGithubOrganizationImmutableReleasesRead,
		Update: resourceGithubOrganizationImmutableReleasesCreateOrUpdate,
		Delete: resourceGithubOrganizationImmutableReleasesDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		CustomizeDiff: customdiff.All(
			diffImmutableReleasesEnforcedRepositories,
		),

		Schema: map[string]*schema.Schema{
			"enforced_repositories": {
				Type:             schema.TypeString,
				Required:         true,
				Description:      "The policy that controls which repositories in the organization have immutable releases enforced. Can be one of: 'all', 'none', or 'selected'.",
				ValidateDiagFunc: validation.ToDiagFunc(validation.StringInSlice([]string{"all", "none", "selected"}, false)),
			},
			"selected_repository_ids": {
				Type:        schema.TypeSet,
				Optional:    true,
				Description: "An array of repository IDs for which immutable releases enforcement should be applied. Only valid when 'enforced_repositories' is set to 'selected'.",
				Elem:        &schema.Schema{Type: schema.TypeInt},
			},
		},
	}
}

func diffImmutableReleasesEnforcedRepositories(_ context.Context, d *schema.ResourceDiff, _ any) error {
	enforced := d.Get("enforced_repositories").(string)
	if enforced != "selected" {
		if _, ok := d.GetOk("selected_repository_ids"); ok {
			return fmt.Errorf("cannot use selected_repository_ids without enforced_repositories being set to selected")
		}
	}
	return nil
}

func resourceGithubOrganizationImmutableReleasesCreateOrUpdate(d *schema.ResourceData, meta any) error {
	client := meta.(*Owner).v3client
	orgName := meta.(*Owner).name
	ctx := context.Background()
	if !d.IsNewResource() {
		ctx = context.WithValue(ctx, ctxId, d.Id())
	}

	err := checkOrganization(meta)
	if err != nil {
		return err
	}

	enforcedRepositories := d.Get("enforced_repositories").(string)

	policy := github.ImmutableReleasePolicy{
		EnforcedRepositories: &enforcedRepositories,
	}

	if enforcedRepositories == "selected" {
		repoIDs, err := expandImmutableReleaseSelectedRepositoryIDs(d)
		if err != nil {
			return err
		}
		policy.SelectedRepositoryIDs = repoIDs
	}

	_, err = client.Organizations.UpdateImmutableReleasesSettings(ctx, orgName, policy)
	if err != nil {
		return fmt.Errorf("error updating immutable releases settings for organization %s: %w", orgName, err)
	}

	d.SetId(orgName)
	return resourceGithubOrganizationImmutableReleasesRead(d, meta)
}

func resourceGithubOrganizationImmutableReleasesRead(d *schema.ResourceData, meta any) error {
	client := meta.(*Owner).v3client
	ctx := context.Background()

	err := checkOrganization(meta)
	if err != nil {
		return err
	}

	settings, _, err := client.Organizations.GetImmutableReleasesSettings(ctx, d.Id())
	if err != nil {
		var ghErr *github.ErrorResponse
		if errors.As(err, &ghErr) && ghErr.Response.StatusCode == http.StatusNotFound {
			log.Printf("[WARN] immutable releases settings not available for organization %s, removing from state", d.Id())
			d.SetId("")
			return nil
		}
		return fmt.Errorf("error reading immutable releases settings for organization %s: %w", d.Id(), err)
	}

	if err = d.Set("enforced_repositories", settings.GetEnforcedRepositories()); err != nil {
		return err
	}

	if settings.GetEnforcedRepositories() == "selected" {
		opts := github.ListOptions{PerPage: 100, Page: 1}
		var repoIDs []int64

		for {
			repos, resp, err := client.Organizations.ListImmutableReleaseRepositories(ctx, d.Id(), &opts)
			if err != nil {
				return fmt.Errorf("error listing immutable release repositories for organization %s: %w", d.Id(), err)
			}
			if repos == nil {
				break
			}
			for _, repo := range repos.Repositories {
				repoIDs = append(repoIDs, repo.GetID())
			}

			if resp.NextPage == 0 {
				break
			}
			opts.Page = resp.NextPage
		}

		if err = d.Set("selected_repository_ids", repoIDs); err != nil {
			return err
		}
	} else {
		if err = d.Set("selected_repository_ids", []int64{}); err != nil {
			return err
		}
	}

	return nil
}

func resourceGithubOrganizationImmutableReleasesDelete(d *schema.ResourceData, meta any) error {
	client := meta.(*Owner).v3client
	orgName := meta.(*Owner).name
	ctx := context.WithValue(context.Background(), ctxId, d.Id())

	err := checkOrganization(meta)
	if err != nil {
		return err
	}

	log.Printf("[WARN] Disabling immutable releases for organization %s. This removes supply chain security protections.", orgName)

	none := "none"
	_, err = client.Organizations.UpdateImmutableReleasesSettings(ctx, orgName, github.ImmutableReleasePolicy{
		EnforcedRepositories: &none,
	})
	if err != nil {
		return fmt.Errorf("error disabling immutable releases for organization %s: %w", orgName, err)
	}

	return nil
}

func expandImmutableReleaseSelectedRepositoryIDs(d *schema.ResourceData) ([]int64, error) {
	raw, ok := d.GetOk("selected_repository_ids")
	if !ok {
		return nil, errors.New("selected_repository_ids must be specified when enforced_repositories is set to 'selected'")
	}

	set := raw.(*schema.Set)
	ids := make([]int64, 0, set.Len())
	for _, v := range set.List() {
		ids = append(ids, int64(v.(int)))
	}

	return ids, nil
}
