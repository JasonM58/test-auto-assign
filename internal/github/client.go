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

	keyInput := cfg.GetGithubPrivateKey()
	if keyInput == "" {
		keyInput = cfg.GetGithubPrivateKeyPath()
	}
	if keyInput == "" {
		log.Fatal("❌ Neither private_key nor private_key_path is configured")
	}

	privateKey, err := githubclient.LoadPrivateKey(keyInput)
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
