package app

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	internalautoassign "github.com/ionextai/git-beacon/internal/auto-assign"
	internalgithub "github.com/ionextai/git-beacon/internal/github"
	internallark "github.com/ionextai/git-beacon/internal/lark"
	internaltelemetry "github.com/ionextai/git-beacon/internal/telemetry"
	"github.com/ionextai/git-beacon/pkg/config"
	"github.com/ionextai/git-beacon/pkg/githubclient"
)

const locationName = "Asia/Jakarta"

func Run(ctx context.Context, cfg *config.Loader, mode string) {

	client := internalgithub.SetupClient(ctx, cfg)

	metrics := &internalautoassign.GitHubMetrics{
		Client: client,
	}

	org := "ionextai"

	// =========================
	// MODE: METRICS ONLY (LOCAL TEST)
	// =========================
	if mode == "metrics" {
		fmt.Println("=== METRICS MODE ===")

		collaborators, _, err := client.Repositories.ListCollaborators(ctx, org, "repo-name", nil)
		if err != nil {
			log.Printf("failed get collaborators: %v", err)
			return
		}

		for _, c := range collaborators {
			user := c.GetLogin()

			if user == "" {
				continue
			}

			openPR, err := metrics.GetOpenPRCount(ctx, org, user)
			if err != nil {
				fmt.Printf("❌ %s error: %v\n", user, err)
				continue
			}

			score := internalautoassign.CalculateScore(openPR, 0)

			fmt.Printf("User=%s OpenPR=%d Score=%d\n", user, openPR, score)
		}

		return
	}

	// =========================
	// MODE: AUTO ASSIGN (DEFAULT)
	// =========================
	fmt.Println("=== AUTO ASSIGN MODE ===")

	autoAssignService := internalautoassign.AutoAssignService{
		Github:  client,
		Metrics: metrics,
	}

	err := autoAssignService.HandlePREvent(ctx)
	if err != nil {
		log.Printf("Auto assign failed: %v", err)
	}

	// =========================
	// TELEMETRY + NOTIFICATION
	// =========================
	shutdown := internaltelemetry.Setup(ctx, cfg)
	defer shutdown(ctx)

	notifier := internallark.NewNotifier(cfg, client)

	loc, _ := time.LoadLocation(locationName)
	today := time.Now().In(loc).Truncate(24 * time.Hour)
	tomorrow := today.Add(24 * time.Hour)

	prs := internalgithub.FetchPRsByOrg(ctx, client, org, today, tomorrow)

	prCounts, err := githubclient.CountPRsByOrgWithRepo(ctx, client, org)
	if err != nil {
		log.Printf("Failed to get PR counts: %v", err)
	}

	sort.Slice(prs, func(i, j int) bool {
		if prs[i].IsOpen != prs[j].IsOpen {
			return prs[i].IsOpen && !prs[j].IsOpen
		}
		return prs[i].CreatedAt.After(prs[j].CreatedAt)
	})

	for _, pr := range prs {

		if pr.IsOpen {
			notifier.NotifyOpenPR(ctx, pr)
			internalgithub.SendMetrics(ctx, client, pr, prCounts)
		} else {
			notifier.NotifyMergedPR(ctx, pr)
			internalgithub.SendMetrics(ctx, client, pr, prCounts)
		}
	}
}
