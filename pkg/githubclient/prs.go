package githubclient

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v61/github"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type PRMetrics struct {
	Number              int
	RepoName            string
	Title               string
	CreatedAt           time.Time
	MergedAt            time.Time
	TimeToFirstReview   time.Duration
	TimeToApproval      time.Duration
	TimeToMerge         time.Duration
	TimeFromApprovalToMerge         time.Duration
	ReviewIterations    int
	SizeCategory string `json:"size_category"`
	PRUrl                   string `json:"pr_url"`
	CreatedBy               string `json:"created_by"`
	Additions     int
	Deletions     int
	ChangedFiles  int
	LOCChanged               int
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

func ListMergedPRs(ctx context.Context, client *github.Client, owner, repo string, since, until time.Time) ([]*github.Issue, error) {
	query := fmt.Sprintf(
		"repo:%s/%s is:pr is:merged merged:%s..%s",
		owner,
		repo,
		since.UTC().Format(time.RFC3339),
		until.UTC().Format(time.RFC3339),
	)

	opts := &github.SearchOptions{
		Sort:        "updated",
		Order:       "desc",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var allPRs []*github.Issue

	for {
		result, resp, err := client.Search.Issues(ctx, query, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to search merged PRs: %w", err)
		}

		allPRs = append(allPRs, result.Issues...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allPRs, nil
}

func ListOpenPRs(ctx context.Context, client *github.Client, owner, repo string) ([]*github.Issue, error) {
	query := fmt.Sprintf("repo:%s/%s is:pr is:open", owner, repo)

	opts := &github.SearchOptions{
		Sort:        "updated",
		Order:       "desc",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var allPRs []*github.Issue

	for {
		result, resp, err := client.Search.Issues(ctx, query, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to search open PRs: %w", err)
		}

		allPRs = append(allPRs, result.Issues...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allPRs, nil
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

func SendPRMetricsToOTel(ctx context.Context, m *PRMetrics) error {
	meter := otel.GetMeterProvider().Meter("github-metrics")

	prAddCounter, _ := meter.Int64Counter("pr_additions_total")
	prDelCounter, _ := meter.Int64Counter("pr_deletions_total")
	prLOCChangedGauge, _ := meter.Int64Gauge("pr_loc_changed")
	prDurationGauge, _ := meter.Float64Gauge("pr_time_to_merge_seconds")

	attrs := []attribute.KeyValue{
		attribute.String("repo", m.RepoName),
		attribute.String("author", m.CreatedBy),
		attribute.String("size_category", m.SizeCategory),
	}

	prAddCounter.Add(ctx, int64(m.Additions), metric.WithAttributes(attrs...))
	prDelCounter.Add(ctx, int64(m.Deletions), metric.WithAttributes(attrs...))
	prLOCChangedGauge.Record(ctx, int64(m.LOCChanged), metric.WithAttributes(attrs...))
	prDurationGauge.Record(ctx, m.TimeToMerge.Seconds(), metric.WithAttributes(attrs...))

	return nil
}
// 	return map[string]interface{}{
// 		"tag":            "column",
// 		"width":          "weighted",
// 		"weight":         1,
// 		"vertical_align": "top",
// 		"elements": []interface{}{
// 			map[string]interface{}{
// 				"tag":              "column_set",
// 				"flex_mode":        "none",
// 				"background_style": "grey",
// 				"columns": []interface{}{
// 					map[string]interface{}{
// 						"tag":            "column",
// 						"width":          "weighted",
// 						"weight":         1,
// 						"vertical_align": "top",
// 						"elements": []interface{}{
// 							map[string]interface{}{
// 								"tag":        "markdown",
// 								"content":    fmt.Sprintf("%s\n<font color='green'>%s</font>\n", title, value),
// 								"text_align": "center",
// 							},
// 						},
// 					},
// 				},
// 			},
// 		},
// 	}
// }