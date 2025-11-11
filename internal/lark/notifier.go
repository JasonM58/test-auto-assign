package lark

import (
	"context"
	"log"

	"github.com/google/go-github/v61/github"
	internalgithub "github.com/ionextai/git-beacon/internal/github"
	"github.com/ionextai/git-beacon/pkg/config"
	"github.com/ionextai/git-beacon/pkg/githubclient"
	"github.com/ionextai/git-beacon/pkg/lark"
)

type Notifier struct {
	webhook     string
	secret      string
	tenantToken string
	emailMap    map[string]string
	ghClient    *github.Client
}

func NewNotifier(cfg *config.Loader, ghClient *github.Client) *Notifier {
	token, err := lark.GetTenantAccessToken(cfg.GetLarkAppID(), cfg.GetLarkAppSecret())
	if err != nil {
		log.Fatalf("❌ Failed to get tenant token: %v", err)
	}
	return &Notifier{
		webhook:     cfg.GetLarkWebhookURL(),
		secret:      cfg.GetLarkSecret(),
		tenantToken: token,
		emailMap:    cfg.GetLarkGithubToEmailMap(),
		ghClient:    ghClient,
	}
}

func (n *Notifier) NotifyOpenPR(ctx context.Context, pr internalgithub.PullRequest) {
	emails := mapReviewersToEmails(pr.AllReviewers, n.emailMap)

	if len(emails) == 0 {
		log.Printf("⚠️ Skipping PR #%d (%s): no reviewers with mapped emails", pr.Number, pr.URL)
		return
	}

	openIDMap, _ := lark.FetchLarkUserMap(n.tenantToken, emails)

	openIDs := make([]string, 0, len(emails))
	for _, email := range emails {
		if id, ok := openIDMap[email]; ok && id != "" {
			openIDs = append(openIDs, id)
		} else {
			log.Printf("⚠️ No Lark user found for reviewer email: %s (PR #%d)", email, pr.Number)
		}
	}

	if len(openIDs) == 0 {
		log.Printf("⚠️ Skipping PR #%d (%s): no valid Lark user IDs found", pr.Number, pr.URL)
		return
	}

	msg := lark.BuildReminderMessage(
		openIDs,
		pr.Author,
		pr.AllReviewers,
		pr.CreatedAt,
		pr.URL,
		pr.Repo.Name,
	)

	if err := lark.SendLarkWebhookMessage(n.webhook, n.secret, msg); err != nil {
		log.Printf("❌ Failed to send Lark reminder for PR #%d: %v", pr.Number, err)
	}
}

func (n *Notifier) NotifyMergedPR(ctx context.Context, pr internalgithub.PullRequest) {
	metrics, err := githubclient.GetPRMetrics(ctx, n.ghClient, pr.Repo.Owner, pr.Repo.Name, pr.RawPR)
	if err != nil {
		log.Printf("❌ Failed to calculate metrics for PR #%d: %v", pr.Number, err)
		return
	}

	reportCard := lark.BuildPRReportCard(metrics)
	if err := lark.SendLarkCard(n.secret, n.webhook, reportCard); err != nil {
		log.Printf("❌ Failed to send Lark report card for PR #%d: %v", pr.Number, err)
	}
}

func mapReviewersToEmails(reviewers []string, emailMap map[string]string) []string {
	var emails []string
	for _, gh := range reviewers {
		if email, ok := emailMap[gh]; ok {
			emails = append(emails, email)
		}
	}
	return emails
}
