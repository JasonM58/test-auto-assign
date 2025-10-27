package github

import (
	"context"
	"log"

	"github.com/google/go-github/v61/github"
	"github.com/ionextai/git-beacon/pkg/config"
	"github.com/ionextai/git-beacon/pkg/githubclient"
	"golang.org/x/oauth2"
)

func SetupClient(ctx context.Context, cfg *config.Loader) *github.Client {
	auth := githubclient.GitHubAuth{AppID: cfg.GetGithubAppId()}

	privateKey, err := githubclient.LoadPrivateKey(cfg.GetGithubPrivateKey())
	if err != nil {
		log.Fatalf("❌ Failed to load private key: %v", err)
	}
	auth.PrivateKey = privateKey

	token, err := auth.GetInstallationToken(ctx)
	if err != nil {
		log.Fatalf("❌ Failed to get installation token: %v", err)
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	return github.NewClient(tc)
}
