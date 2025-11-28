package githubclient

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/go-github/v61/github"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type PRMetrics struct {
	Number                  int
	RepoName                string
	Title                   string
	CreatedAt               time.Time
	MergedAt                time.Time
	TimeToFirstReview       time.Duration
	TimeToApproval          time.Duration
	TimeToMerge             time.Duration
	TimeFromApprovalToMerge time.Duration
	ReviewIterations        int
	SizeCategory            string `json:"size_category"`
	PRUrl                   string `json:"pr_url"`
	CreatedBy               string `json:"created_by"`
	Additions               int
	Deletions               int
	ChangedFiles            int
	LOCChanged              int
}

func GetPRMetrics(ctx context.Context, client *github.Client, owner, repo string, pr *github.PullRequest) (*PRMetrics, error) {
	prNumber := pr.GetNumber()
	createdAt := pr.GetCreatedAt()
	mergedAt := pr.GetMergedAt()

	// Fetch all reviews
	reviews, _, err := client.PullRequests.ListReviews(ctx, owner, repo, prNumber, nil)
	if err != nil {
		return nil, err
	}

	var (
		firstReviewTime  *time.Time
		approvalTime     *time.Time
		reviewIterations int
	)

	for _, review := range reviews {
		submittedAt := review.SubmittedAt.Time

		if review.GetState() == "COMMENTED" || review.GetState() == "CHANGES_REQUESTED" || review.GetState() == "APPROVED" {
			if firstReviewTime == nil || submittedAt.Before(*firstReviewTime) {
				firstReviewTime = &submittedAt
			}
		}

		if review.GetState() == "APPROVED" && approvalTime == nil {
			approvalTime = &submittedAt
		}

		if review.GetState() == "CHANGES_REQUESTED" || review.GetState() == "COMMENTED" {
			reviewIterations++
		}
	}

	timeToMerge := mergedAt.Sub(createdAt.Time)
	var timeToFirstReview, timeToApproval, timeFromApprovalToMerge time.Duration

	if firstReviewTime != nil {
		timeToFirstReview = firstReviewTime.Sub(createdAt.Time)
	}
	if approvalTime != nil {
		timeToApproval = approvalTime.Sub(createdAt.Time)
	}
	if approvalTime != nil && !mergedAt.IsZero() {
		timeFromApprovalToMerge = timeToMerge - timeToApproval
	}

	sizeCategory := categorizePRSize(pr)
	locChanged := pr.GetAdditions() + pr.GetDeletions()

	metrics := &PRMetrics{
		Number:                  prNumber,
		RepoName:                repo,
		Title:                   pr.GetTitle(),
		CreatedAt:               createdAt.Time,
		MergedAt:                mergedAt.Time,
		TimeToFirstReview:       timeToFirstReview,
		TimeToApproval:          timeToApproval,
		TimeToMerge:             timeToMerge,
		TimeFromApprovalToMerge: timeFromApprovalToMerge,
		ReviewIterations:        reviewIterations,
		SizeCategory:            sizeCategory,
		PRUrl:                   pr.GetHTMLURL(),
		CreatedBy:               pr.GetUser().GetLogin(),
		Additions:               pr.GetAdditions(),
		Deletions:               pr.GetDeletions(),
		ChangedFiles:            pr.GetChangedFiles(),
		LOCChanged:              locChanged,
	}

	return metrics, nil
}

func ListOpenPRsByOrg(ctx context.Context, client *github.Client, org string) ([]*github.Issue, error) {
	query := fmt.Sprintf("org:%s is:pr is:open", org)
	return searchIssues(ctx, client, query)
}

func ListMergedPRsByOrg(ctx context.Context, client *github.Client, org string, since, until time.Time) ([]*github.Issue, error) {
	query := fmt.Sprintf(
		"org:%s is:pr is:merged merged:%s..%s",
		org,
		since.UTC().Format("2006-01-02"),
		until.UTC().Format("2006-01-02"),
	)
	return searchIssues(ctx, client, query)
}

func categorizePRSize(pr *github.PullRequest) string {
	totalChanges := pr.GetAdditions() + pr.GetDeletions()

	switch {
	case totalChanges < 50:
		return "small"
	case totalChanges < 200:
		return "medium"
	case totalChanges < 500:
		return "large"
	default:
		return "extra large"
	}
}

func SendPRMetricsToOTel(ctx context.Context, m *PRMetrics, client *github.Client) error {
	meter := otel.GetMeterProvider().Meter("github-metrics")

	prAddCounter, _ := meter.Int64Counter("pr_additions_total")
	prDelCounter, _ := meter.Int64Counter("pr_deletions_total")
	prLOCChangedGauge, _ := meter.Int64Gauge("pr_loc_changed")
	prDurationGauge, _ := meter.Float64Gauge("pr_time_to_merge_seconds")
	prFirstTimeToReview, _ := meter.Float64Gauge("pr_time_to_first_review_seconds")
	prApproveToMergeTime, _ := meter.Float64Gauge("pr_time_from_approval_to_merge_seconds")
	prReviewIterations, _ := meter.Int64Gauge("pr_review_iterations")
	prAprovalTime, _ := meter.Float64Gauge("pr_time_to_approval_seconds")
	prCount, err := countPRsByOrgWithRepo(ctx, client, Organization)
	if err != nil {
		log.Printf("Failed to get PR counts: %v", err)
	}

	attrs := []attribute.KeyValue{
		attribute.String("repo", m.RepoName),
		attribute.String("author", m.CreatedBy),
		attribute.String("size_category", m.SizeCategory),
	}

	prAddCounter.Add(ctx, int64(m.Additions), metric.WithAttributes(attrs...))
	prDelCounter.Add(ctx, int64(m.Deletions), metric.WithAttributes(attrs...))
	prLOCChangedGauge.Record(ctx, int64(m.LOCChanged), metric.WithAttributes(attrs...))
	prDurationGauge.Record(ctx, m.TimeToMerge.Seconds(), metric.WithAttributes(attrs...))
	prFirstTimeToReview.Record(ctx, m.TimeToFirstReview.Seconds(), metric.WithAttributes(attrs...))
	prApproveToMergeTime.Record(ctx, m.TimeFromApprovalToMerge.Seconds(), metric.WithAttributes(attrs...))
	prReviewIterations.Record(ctx, int64(m.ReviewIterations), metric.WithAttributes(attrs...))
	prAprovalTime.Record(ctx, m.TimeToApproval.Seconds(), metric.WithAttributes(attrs...))
	emitPRCountMetrics(ctx, prCount, m)

	return nil
}

func searchIssues(ctx context.Context, client *github.Client, query string) ([]*github.Issue, error) {
	opts := &github.SearchOptions{
		Sort:        "updated",
		Order:       "desc",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var all []*github.Issue
	for {
		result, resp, err := client.Search.Issues(ctx, query, opts)
		if err != nil {
			if resp != nil && resp.Rate.Remaining == 0 {
				wait := time.Until(resp.Rate.Reset.Time)
				log.Printf("⚠️ Rate limit hit. Waiting %v...", wait)
				time.Sleep(wait + time.Second)
				continue
			}
			return nil, err
		}
		all = append(all, result.Issues...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return all, nil
}

func countPRsByOrgWithRepo(ctx context.Context, client *github.Client, org string) (map[string]map[string]int, error) {
	result := make(map[string]map[string]int)

	opts := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var allRepos []*github.Repository
	for {
		repos, resp, err := client.Repositories.ListByOrg(ctx, org, opts)
		if err != nil {
			return nil, err
		}
		allRepos = append(allRepos, repos...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	for _, repo := range allRepos {
		name := repo.GetName()
		result[name] = map[string]int{
			"open":   0,
			"merged": 0,
		}

		openOpts := &github.PullRequestListOptions{
			State:       "open",
			ListOptions: github.ListOptions{PerPage: 1},
		}
		_, resp, err := client.PullRequests.List(ctx, org, name, openOpts)
		if err != nil {
			log.Printf("Warning: failed to count open PRs for %s: %v", name, err)
			continue
		}
		if resp.LastPage > 0 {
			result[name]["open"] = resp.LastPage * openOpts.PerPage
		} else if len(resp.Header["Link"]) == 0 {
			prs, _, _ := client.PullRequests.List(ctx, org, name, openOpts)
			result[name]["open"] = len(prs)
		}

		closedOpts := &github.PullRequestListOptions{
			State:       "closed",
			ListOptions: github.ListOptions{PerPage: 100},
		}

		mergedCount := 0
		for {
			prs, resp, err := client.PullRequests.List(ctx, org, name, closedOpts)
			if err != nil {
				log.Printf("Warning: failed to count merged PRs for %s: %v", name, err)
				break
			}

			for _, pr := range prs {
				if pr.MergedAt != nil {
					mergedCount++
				}
			}

			if resp.NextPage == 0 {
				break
			}
			closedOpts.Page = resp.NextPage
		}
		result[name]["merged"] = mergedCount
	}

	return result, nil
}

func emitPRCountMetrics(ctx context.Context, counts map[string]map[string]int, m *PRMetrics) {
	meter := otel.GetMeterProvider().Meter("github-metrics")

	prCountGauge, _ := meter.Int64Gauge("pr_count")

	for repo, statusMap := range counts {
		for status, count := range statusMap {
			prCountGauge.Record(
				ctx,
				int64(count),
				metric.WithAttributes(
					attribute.String("repo", repo),
					attribute.String("merge_status", status),
					attribute.String("author", m.CreatedBy),
					attribute.String("size_category", m.SizeCategory),
				),
			)
		}
	}
}
