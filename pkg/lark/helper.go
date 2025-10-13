package lark

import (
	"fmt"
	"strings"
	"time"

	"github.com/ionextai/git-scrapper/pkg/githubclient"
)

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

func BuildPRReportCard(metrics *githubclient.PRMetrics) map[string]interface{} {
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