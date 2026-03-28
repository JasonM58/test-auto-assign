package autoassign

import (
	"context"
	"fmt"
	"time"

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

	for i := 0; i < 3; i++ {
		result, _, err := g.Client.Search.Issues(ctx, query, nil)

		if err == nil && result != nil {
			return result.GetTotal(), nil
		}

		time.Sleep(1 * time.Second)
	}

	return 0, fmt.Errorf("failed after retry for user=%s query=%s", user, query)
}

func (g *GitHubMetrics) GetRecentReviewCount(ctx context.Context, org, user string) (int, error) {
	query := fmt.Sprintf(
		"org:%s is:pr reviewed-by:%s updated:>7d",
		org,
		user,
	)

	for i := 0; i < 3; i++ {
		result, _, err := g.Client.Search.Issues(ctx, query, nil)

		if err == nil && result != nil {
			return result.GetTotal(), nil
		}

		fmt.Printf("[WARN] recent retry %d user=%s err=%v\n", i+1, user, err)
		time.Sleep(time.Duration(i+1) * time.Second)
	}

	return 0, fmt.Errorf("failed recent reviews user=%s", user)
}
