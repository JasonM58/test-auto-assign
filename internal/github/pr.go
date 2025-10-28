package github

import (
	"context"
	"log"
	"time"

	"github.com/google/go-github/v61/github"
	"github.com/ionextai/git-beacon/pkg/githubclient"
)

type Repository struct {
	Owner string
	Name  string
}

type PullRequest struct {
	Number    int
	URL       string
	Author    string
	IsOpen    bool
	CreatedAt time.Time
	ClosedAt  *time.Time
	Repo      Repository
	RawPR     *github.PullRequest
}

func FetchRepositories(ctx context.Context, client *github.Client) []Repository {
	repos, _, err := client.Apps.ListRepos(ctx, &github.ListOptions{PerPage: 100})
	if err != nil {
		log.Fatalf("❌ Failed to list installation repos: %v", err)
	}

	var list []Repository
	for _, r := range repos.Repositories {
		list = append(list, Repository{
			Owner: r.GetOwner().GetLogin(),
			Name:  r.GetName(),
		})
	}
	return list
}

func FetchPRs(ctx context.Context, client *github.Client, repo Repository, today, tomorrow time.Time) []PullRequest {
	mergedIssues, err := githubclient.ListMergedPRs(ctx, client, repo.Owner, repo.Name, today, tomorrow)
	if err != nil {
		log.Printf("⚠️ Failed to list merged PRs: %v", err)
	}

	openIssues, err := githubclient.ListOpenPRs(ctx, client, repo.Owner, repo.Name)
	if err != nil {
		log.Printf("⚠️ Failed to list open PRs: %v", err)
	}

	all := make([]*github.Issue, 0, len(mergedIssues)+len(openIssues))
	all = append(all, mergedIssues...)
	all = append(all, openIssues...)

	var prs []PullRequest

	for _, issue := range all {
		prDetail, _, err := client.PullRequests.Get(ctx, repo.Owner, repo.Name, issue.GetNumber())
		if err != nil {
			log.Printf("⚠️ Failed to fetch PR #%d details: %v", issue.GetNumber(), err)
			continue
		}

		isOpen := prDetail.GetClosedAt().IsZero()
		prs = append(prs, PullRequest{
			Number:    prDetail.GetNumber(),
			URL:       prDetail.GetHTMLURL(),
			Author:    prDetail.GetUser().GetLogin(),
			IsOpen:    isOpen,
			CreatedAt: prDetail.GetCreatedAt().Time,
			ClosedAt:  getClosedTime(prDetail),
			Repo:      repo,
			RawPR:     prDetail,
		})
	}
	return prs
}

func getClosedTime(pr *github.PullRequest) *time.Time {
	if pr.GetClosedAt().IsZero() {
		return nil
	}
	t := pr.GetClosedAt().Time
	return &t
}

func SendMetrics(ctx context.Context, client *github.Client, pr PullRequest) {
	metrics, err := githubclient.GetPRMetrics(ctx, client, pr.Repo.Owner, pr.Repo.Name, pr.RawPR)
	if err != nil {
		log.Printf("❌ Failed to calculate metrics for PR #%d: %v", pr.Number, err)
		return
	}

	githubclient.SendPRMetricsToOTel(ctx, metrics)
}
