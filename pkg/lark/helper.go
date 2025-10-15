package lark

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ionextai/git-scrapper/pkg/githubclient"
)

type TenantAccessTokenResponse struct {
	Code              int    `json:"code"`
	Msg               string `json:"msg"`
	TenantAccessToken string `json:"tenant_access_token"`
	Expire            int    `json:"expire"`
}

type LarkUser struct {
	Email  string `json:"email"`
	UserID string `json:"user_id"`
}

type larkUserResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		UserList []LarkUser `json:"user_list"`
	} `json:"data"`
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

func BuildReminderMessage(openIDs []string, createdBy string, reviewers []string, createdAt time.Time, prURL, repoName string) map[string]interface{} {
	reviewerList := "(no reviewers)"
	if len(reviewers) > 0 {
		reviewerList = fmt.Sprintf("%s", reviewers[0])
		if len(reviewers) > 1 {
			reviewerList += fmt.Sprintf(" and %d others", len(reviewers)-1)
		}
	}

	var intro []map[string]interface{}
	intro = append(intro, map[string]interface{}{
		"tag": "text",
		"text": "Hi ",
	})
	for _, id := range openIDs {
		intro = append(intro, map[string]interface{}{
			"tag":     "at",
			"user_id": id,
		})
		intro = append(intro, map[string]interface{}{
			"tag": "text",
			"text": " ",
		})
	}
	intro = append(intro, map[string]interface{}{
		"tag": "text",
		"text": "👋\n\nYou have a pending PR in ",
	})
	intro = append(intro, map[string]interface{}{
		"tag": "text",
		"text": fmt.Sprintf("%s", repoName),
	})
	intro = append(intro, map[string]interface{}{
		"tag": "text",
		"text": " that needs your review.\n\n",
	})

	content := [][]map[string]interface{}{
		intro,
		{
			{
				"tag": "text",
				"text": fmt.Sprintf(
					"Created By: %s\nReviewer(s): %s\nOpened For: %s\n\n",
					createdBy,
					reviewerList,
					FormatDuration(time.Since(createdAt)),
				),
			},
		},
		{
			{
				"tag":  "a",
				"text": "👉 View Pull Request",
				"href": prURL,
			},
		},
	}

	// Return final message body
	return map[string]interface{}{
		"msg_type": "post",
		"content": map[string]interface{}{
			"post": map[string]interface{}{
				"en_us": map[string]interface{}{
					"title":   "🔔 Pull Request Reminder",
					"content": content,
				},
			},
		},
	}
}

func FetchLarkUserMap(tenantAccessToken string, emails []string) (map[string]string, error) {
	if len(emails) == 0 {
		return nil, fmt.Errorf("no emails provided")
	}

	url := "https://open.larksuite.com/open-apis/contact/v3/users/batch_get_id?user_id_type=open_id"

	body, _ := json.Marshal(map[string]interface{}{
		"emails": emails,
	})

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+tenantAccessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Lark API: %w", err)
	}
	defer resp.Body.Close()

	var result larkUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode Lark API response: %w", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("Lark API error: %s (code: %d)", result.Msg, result.Code)
	}

	mapping := make(map[string]string)
	for _, user := range result.Data.UserList {
		mapping[user.Email] = user.UserID
	}

	return mapping, nil
}

func GetTenantAccessToken(appID, appSecret string) (string, error) {
	url := "https://open.larksuite.com/open-apis/auth/v3/tenant_access_token/internal"

	body, _ := json.Marshal(map[string]string{
		"app_id":     appID,
		"app_secret": appSecret,
	})

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to request Lark token: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp TenantAccessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode Lark token response: %w", err)
	}

	if tokenResp.Code != 0 {
		return "", fmt.Errorf("Lark token error: %s (code %d)", tokenResp.Msg, tokenResp.Code)
	}

	return tokenResp.TenantAccessToken, nil
}