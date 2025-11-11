package github

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/google/go-github/v61/github"
	"github.com/ionextai/git-beacon/pkg/githubclient"
)

type Repository struct {
	Owner string
	Name  string
}

type PullRequest struct {
	Number       int
	URL          string
	Author       string
	IsOpen       bool
	CreatedAt    time.Time
	ClosedAt     *time.Time
	Repo         Repository
	RawPR        *github.PullRequest
	AllReviewers []string
}

func getUniqueReviewers(ctx context.Context, client *github.Client, prDetail *github.PullRequest) []string {
	requestedUsers := make([]string, 0, len(prDetail.RequestedReviewers))
	for _, user := range prDetail.RequestedReviewers {
		requestedUsers = append(requestedUsers, user.GetLogin())
	}

	requestedTeams := make([]string, 0, len(prDetail.RequestedTeams))
	for _, team := range prDetail.RequestedTeams {
		requestedTeams = append(requestedTeams, team.GetName())
	}

	owner := prDetail.GetBase().GetRepo().GetOwner().GetLogin()
	repoName := prDetail.GetBase().GetRepo().GetName()
	reviews, _, _ := client.PullRequests.ListReviews(ctx, owner, repoName, prDetail.GetNumber(), nil)

	actualReviewers := make([]string, 0, len(reviews))
	for _, review := range reviews {
		if review.User != nil {
			actualReviewers = append(actualReviewers, review.User.GetLogin())
		}
	}

	unique := make(map[string]bool)
	for _, n := range append(append(requestedUsers, requestedTeams...), actualReviewers...) {
		unique[n] = true
	}

	allReviewers := make([]string, 0, len(unique))
	for n := range unique {
		if n == prDetail.GetUser().GetLogin() {
			continue
		}
		allReviewers = append(allReviewers, n)
	}

	return allReviewers
}

func FetchPRsByOrg(ctx context.Context, client *github.Client, org string, today, tomorrow time.Time) []PullRequest {
	openIssues, err := githubclient.ListOpenPRsByOrg(ctx, client, org)
	if err != nil {
		log.Printf("Failed to list open PRs for org %s: %v", org, err)
	}

	mergedIssues, err := githubclient.ListMergedPRsByOrg(ctx, client, org, today, tomorrow)
	if err != nil {
		log.Printf("Failed to list merged PRs for org %s: %v", org, err)
	}

	all := append(mergedIssues, openIssues...)
	prs := make([]PullRequest, 0, len(all))

	for _, issue := range all {
		repoURL := issue.GetRepositoryURL()
		parts := strings.Split(repoURL, "/")
		if len(parts) < 2 {
			continue
		}
		owner := parts[len(parts)-2]
		repoName := parts[len(parts)-1]

		prDetail, _, err := client.PullRequests.Get(ctx, owner, repoName, issue.GetNumber())
		if err != nil {
			if _, ok := err.(*github.RateLimitError); ok {
				limits, _, _ := client.RateLimit.Get(ctx)
				if limits != nil && limits.Core != nil {
					reset := time.Until(limits.Core.Reset.Time)
					log.Printf("Rate limit hit, sleeping for %v...", reset)
					time.Sleep(reset + time.Second)
					continue
				}
			}
			log.Printf("Failed to fetch PR #%d (%s/%s): %v", issue.GetNumber(), owner, repoName, err)
			continue
		}

		if prDetail.GetDraft(){
			continue
		}

		allReviewers := getUniqueReviewers(ctx, client, prDetail)

		prs = append(prs, PullRequest{
			Number:       prDetail.GetNumber(),
			URL:          prDetail.GetHTMLURL(),
			Author:       prDetail.GetUser().GetLogin(),
			IsOpen:       prDetail.GetClosedAt().IsZero(),
			CreatedAt:    prDetail.GetCreatedAt().Time,
			ClosedAt:     getClosedTime(prDetail),
			Repo:         Repository{Owner: owner, Name: repoName},
			AllReviewers: allReviewers,
			RawPR:        prDetail,
		})
	}

	return prs
}

func FetchRepositories(ctx context.Context, client *github.Client) []Repository {
	repos, _, err := client.Apps.ListRepos(ctx, &github.ListOptions{PerPage: 100})
	if err != nil {
		log.Fatalf("❌ Failed to list installation repos: %v", err)
	}

	list := make([]Repository, 0, len(repos.Repositories))

	for _, r := range repos.Repositories {
		list = append(list, Repository{
			Owner: r.GetOwner().GetLogin(),
			Name:  r.GetName(),
		})
	}

	return list
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
