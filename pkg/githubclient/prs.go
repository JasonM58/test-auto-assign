package githubclient

import (
	"context"
	"fmt"
	"strings"
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

func FormatMetricsMessage(metrics *PRMetrics) string {
	return fmt.Sprintf(
		"🏷️ Repository: %s\n"+
			"📦 PR #%d: %s\n"+
			"👤 Author: %s\n"+
			"🔗 URL: %s\n"+
			"📅 Created At: %s\n"+
			"📅 Merged At: %s\n"+
			"📊 Size Category: %s\n\n"+
			"⏱️ Time to First Review: %s\n"+
			"✅ Time to Approval: %s\n"+
			"🔀 Time to Merge: %s\n"+
			"🕒 Time From Approval to Merge: %s\n"+
			"🔁 Review Iterations: %d\n\n"+
			"📈 Additions: %d | 🗑️ Deletions: %d | 🧩 Changed Files: %d | 📏 LOC Changed: %d",
		metrics.RepoName,
		metrics.Number,
		metrics.Title,
		metrics.CreatedBy,
		metrics.PRUrl,
		metrics.CreatedAt.Format("2006-01-02 15:04:05"),
		metrics.MergedAt.Format("2006-01-02 15:04:05"),
		metrics.SizeCategory,
		metrics.TimeToFirstReview,
		metrics.TimeToApproval,
		metrics.TimeToMerge,
		metrics.TimeFromApprovalToMerge,
		metrics.ReviewIterations,
		metrics.Additions,
		metrics.Deletions,
		metrics.ChangedFiles,
		metrics.LOCChanged,
	)
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

func FormatShortDuration(d time.Duration) string {
    if d < 0 {
        d = -d
    }

    hours := int(d.Hours())
    minutes := int(d.Minutes()) % 60
    seconds := int(d.Seconds()) % 60

    switch {
    case hours > 0:
        return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
    case minutes > 0:
        return fmt.Sprintf("%dm %ds", minutes, seconds)
    default:
        return fmt.Sprintf("%ds", seconds)
    }
}

func FormatDuration(d time.Duration) string {
    days := int(d.Hours()) / 24
    hours := int(d.Hours()) % 24
    minutes := int(d.Minutes()) % 60

    switch {
    case days > 0:
        return fmt.Sprintf("%d days %d hours", days, hours)
    case hours > 0:
        return fmt.Sprintf("%d hours %d minutes", hours, minutes)
    case minutes > 0:
        return fmt.Sprintf("%d minutes", minutes)
    default:
        return "Just Now"
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


func BuildPRReportCard(metrics *PRMetrics) map[string]interface{} {
	return map[string]interface{}{
		"config": map[string]interface{}{
			"wide_screen_mode": true,
		},
		"elements": []interface{}{
			map[string]interface{}{
				"tag": "markdown",
				"content": fmt.Sprintf(
					"🏠 **Repository:** %s\n"+
						"🏷️ **Title:** %s\n"+
						"👤 **Created by:** %s\n"+
						"📊 **LOC Changed:** %d lines\n\n"+
						"⏱️ **Time to First Review:** %s\n"+
						"📝 **Time to Approval:** %s\n"+
						"⏩ **Time to Merge:** %s\n"+
						"✅ **Time From Approval to Merge:** %s\n"+
						"🔄 **Review Iterations:** %d",
					metrics.RepoName,
					metrics.Title,
					metrics.CreatedBy,
					metrics.LOCChanged,
					FormatDuration(metrics.TimeToFirstReview),
					FormatDuration(metrics.TimeToApproval),
					FormatDuration(metrics.TimeToMerge),
					FormatShortDuration(metrics.TimeFromApprovalToMerge),
					metrics.ReviewIterations,
				),
			},
			map[string]interface{}{
				"tag": "action",
				"actions": []interface{}{
					map[string]interface{}{
						"tag": "button",
						"text": map[string]interface{}{
							"tag":     "plain_text",
							"content": "View Detail",
						},
						"type": "primary",
						"url":  metrics.PRUrl,
					},
				},
			},
		},
		"header": map[string]interface{}{
			"template": "blue",
			"title": map[string]interface{}{
				"content": "🗞️ Pull Request Report",
				"tag":     "plain_text",
			},
		},
	}
}

func BuildReminderCard(createdBy string, reviewers []string, createdAt time.Time, prURL string, repoName string) map[string]interface{} {
	reviewerList := "(no reviewers)"
	if len(reviewers) > 0 {
		reviewerList = strings.Join(reviewers, ", ")
	}

	return map[string]interface{}{
		"config": map[string]interface{}{
			"wide_screen_mode": true,
		},
		"elements": []interface{}{
			map[string]interface{}{
				"tag": "div",
				"text": map[string]interface{}{
					"content": "You have pending pull requests that need your attention. Please review or merge them to keep development on track.",
					"tag":     "lark_md",
				},
			},
			map[string]interface{}{
				"tag":              "column_set",
				"flex_mode":        "none",
				"background_style": "default",
				"columns": []interface{}{
					columnItem("**Repository**", repoName),
					columnItem("**Created By**", createdBy),
					columnItem("**Reviewer**", reviewerList),
					columnItem("**Opened For**", FormatDuration(time.Since(createdAt))),
				},
			},
			map[string]interface{}{
				"tag": "action",
				"actions": []interface{}{
					map[string]interface{}{
						"tag":  "button",
						"text": map[string]interface{}{"content": "View Pull Request", "tag": "plain_text"},
						"url":  prURL,
						"type": "primary",
					},
				},
			},
		},
		"header": map[string]interface{}{
			"template": "yellow",
			"title": map[string]interface{}{
				"content": "🔔 Pull Request Reminder",
				"tag":     "plain_text",
			},
		},
	}
}

func columnItem(title, value string) map[string]interface{} {
	return map[string]interface{}{
		"tag":            "column",
		"width":          "weighted",
		"weight":         1,
		"vertical_align": "top",
		"elements": []interface{}{
			map[string]interface{}{
				"tag":              "column_set",
				"flex_mode":        "none",
				"background_style": "grey",
				"columns": []interface{}{
					map[string]interface{}{
						"tag":            "column",
						"width":          "weighted",
						"weight":         1,
						"vertical_align": "top",
						"elements": []interface{}{
							map[string]interface{}{
								"tag":        "markdown",
								"content":    fmt.Sprintf("%s\n<font color='green'>%s</font>\n", title, value),
								"text_align": "center",
							},
						},
					},
				},
			},
		},
	}
}