package autoassign

import (
	"context"

	"github.com/google/go-github/v61/github"
)

type MetricsProvider interface {
	GetOpenPRCount(ctx context.Context, org, user string) (int, error)
	GetRecentReviewCount(ctx context.Context, org, user string) (int, error)
}

type GitHubMetrics struct {
	Client *github.Client
}
