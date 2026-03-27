package app

import (
	"context"
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

func Run(ctx context.Context, cfg *config.Loader) {

	loc, _ := time.LoadLocation(locationName)

	today := time.Now().In(loc).Truncate(24 * time.Hour)
	tomorrow := today.Add(24 * time.Hour)

	client := internalgithub.SetupClient(ctx, cfg)

	autoAssignService := internalautoassign.AutoAssignService{
		Github: client,
	}

	err := autoAssignService.HandlePREvent(ctx)
	if err != nil {
		log.Printf("Auto assign failed: %v", err)
	}

	shutdown := internaltelemetry.Setup(ctx, cfg)
	defer shutdown(ctx)

	notifier := internallark.NewNotifier(cfg, client)

	org := "ionextai"

	// fetch PR today (open + closed)
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

			// notify open PR
			notifier.NotifyOpenPR(ctx, pr)

			// reviewer workload metric (OPEN PR)
			internalgithub.SendMetrics(ctx, client, pr, prCounts)

		} else {

			// notify merged PR
			notifier.NotifyMergedPR(ctx, pr)

			internalgithub.SendMetrics(ctx, client, pr, prCounts)

		}
	}
}
