package autoassign

import (
	"context"
	"fmt"

	"github.com/google/go-github/v61/github"
)

type MetricsProvider interface {
	GetOpenPRCount(ctx context.Context, org, user string) (int, error)
	GetRecentReviewCount(ctx context.Context, org, user string) (int, error)
}

type GitHubMetrics struct {
	Client *github.Client
}

func (g *GitHubMetrics) GetOpenPRCount(ctx context.Context, org, user string) (int, error) {
	query := fmt.Sprintf("org:%s is:pr is:open review-requested:%s", org, user)

	result, _, err := g.Client.Search.Issues(ctx, query, nil)
	if err != nil {
		return 0, err
	}

	return result.GetTotal(), nil
}

func (g *GitHubMetrics) GetRecentReviewCount(ctx context.Context, org, user string) (int, error) {
	// sementara dummy
	return 0, nil
}
