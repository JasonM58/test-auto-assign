package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/google/go-github/v61/github"
	"github.com/ionextai/git-scrapper/pkg/config"
	"github.com/ionextai/git-scrapper/pkg/githubclient"
	"github.com/ionextai/git-scrapper/pkg/lark"
	"github.com/ionextai/git-scrapper/pkg/telemetry"
	"golang.org/x/oauth2"
)

var (
	defaultConfigPath = "env.yaml"
)

func main() {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	today := time.Now().In(loc).Truncate(24 * time.Hour)
	tomorrow := today.Add(24 * time.Hour)

	cfgPath := flag.String("config", defaultConfigPath, "config file to load")
	flag.Parse()

	cfg := config.NewConfig(*cfgPath)

	// --- GitHub Auth ---
	auth := githubclient.GitHubAuth{
		AppID: cfg.GetGithubAppId(),
	}
	privateKey, err := githubclient.LoadPrivateKey(cfg.GetGithubPrivateKeyPath())
	if err != nil {
		log.Fatalf("❌ Failed to load private key: %v", err)
	}
	auth.PrivateKey = privateKey

	token, err := auth.GetInstallationToken(context.Background())
	if err != nil {
		log.Fatalf("❌ Failed to get installation token: %v", err)
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	// --- OpenTelemetry Setup ---
	shutdown, err := telemetry.Init(ctx, telemetry.Config{
		ServiceName:          "github-pr-metrics",
		ServiceVer:           "1.0.0",
		Endpoint:             cfg.GetOTLPEndpoint(),
		Insecure:             cfg.GetOTLPInsecure(),
		Environment:          cfg.GetEnvironment(),
		MetricExportInterval: cfg.GetTelemetryMetricExportInterval(),
		Enabled:              cfg.GetTelemetryEnabled(),
	})
	if err != nil {
		log.Fatalf("❌ failed to initialize telemetry: %v", err)
	}
	defer shutdown(ctx)

	// --- Repositories ---
	repos, _, err := client.Apps.ListRepos(ctx, &github.ListOptions{PerPage: 100})
	if err != nil {
		log.Fatalf("❌ failed to list installation repos: %v", err)
	}

	for _, repo := range repos.Repositories {
		owner := repo.GetOwner().GetLogin()
		repoName := repo.GetName()

		fmt.Printf("🔍 Checking repo: %s/%s\n", owner, repoName)

		mergedPRs, err := githubclient.ListMergedPRs(ctx, client, owner, repoName, today, tomorrow)
		if err != nil {
			log.Fatal(err)
		}

		openPRs, err := githubclient.ListOpenPRs(ctx, client, owner, repoName)
		if err != nil {
			log.Fatal(err)
		}

		allPRs := append(mergedPRs, openPRs...)

		for _, pr := range allPRs {
			prDetail, _, err := client.PullRequests.Get(ctx, owner, repoName, pr.GetNumber())
			if err != nil {
				log.Printf("⚠️ Failed to fetch PR #%d details: %v", pr.GetNumber(), err)
				continue
			}

			createdAt := prDetail.GetCreatedAt().Time
			var reviewers []string
			for _, r := range prDetail.RequestedReviewers {
					reviewers = append(reviewers, r.GetLogin())
			}

			reviews, _, err := client.PullRequests.ListReviews(ctx, owner, repoName, prDetail.GetNumber(), nil)
			if err == nil {
					for _, rev := range reviews {
							login := rev.GetUser().GetLogin()
							if login != "" && !githubclient.Contains(reviewers, login) {
									reviewers = append(reviewers, login)
							}
					}
			}

			if len(reviewers) == 0 {
					reviewers = []string{"(no reviewers)"}
			}

			if pr.GetClosedAt().IsZero() {
					// Open PR → Reminder card
					reminderCard := githubclient.BuildReminderCard(
							pr.GetUser().GetLogin(),
							reviewers,
							createdAt,
							pr.GetHTMLURL(),
							repoName,
					)
					if err := lark.SendLarkCard(cfg.GetLarkSecret(), cfg.GetLarkWebhookURL(), reminderCard); err != nil {
							log.Printf("❌ Failed to send reminder card for PR #%d: %v", pr.GetNumber(), err)
					}

			} else {
					metrics, err := githubclient.GetPRMetrics(ctx, client, owner, repoName, prDetail)
					if err != nil {
							log.Printf("❌ Failed to calculate metrics for PR #%d: %v", pr.GetNumber(), err)
							continue
					}

					// Send to OTel
					githubclient.SendPRMetricsToOTel(ctx, metrics)

					// Send report card
					reportCard := githubclient.BuildPRReportCard(metrics)
					if err := lark.SendLarkCard(cfg.GetLarkSecret(), cfg.GetLarkWebhookURL(), reportCard); err != nil {
							log.Printf("❌ Failed to send Lark report card for PR #%d: %v", pr.GetNumber(), err)
					}
			}
		}
	}
}
