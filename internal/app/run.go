package app

import (
	"context"
	"fmt"
	"sort"
	"time"

	internalgithub "github.com/ionextai/git-beacon/internal/github"
	internallark "github.com/ionextai/git-beacon/internal/lark"
	internaltelemetry "github.com/ionextai/git-beacon/internal/telemetry"
	"github.com/ionextai/git-beacon/pkg/config"
)

const locationName = "Asia/Jakarta"

func Run(ctx context.Context, cfg *config.Loader) {
	loc, _ := time.LoadLocation(locationName)
	today := time.Now().In(loc).Truncate(24 * time.Hour)
	tomorrow := today.Add(24 * time.Hour)

	client := internalgithub.SetupClient(ctx, cfg)
	shutdown := internaltelemetry.Setup(ctx, cfg)
	defer shutdown(ctx)

	notifier := internallark.NewNotifier(cfg, client)

	org := "ionextai"
	fmt.Printf("🏢 Fetching all PRs in org")

	allPRs := internalgithub.FetchPRsByOrg(ctx, client, org, today, tomorrow)

	sort.Slice(allPRs, func(i, j int) bool {
			if allPRs[i].IsOpen != allPRs[j].IsOpen {
					return allPRs[i].IsOpen && !allPRs[j].IsOpen
			}
			return allPRs[i].CreatedAt.After(allPRs[j].CreatedAt)
	})

	for _, pr := range allPRs {
		if pr.IsOpen {
				notifier.NotifyOpenPR(ctx, pr)
		} else {
				notifier.NotifyMergedPR(ctx, pr)
				internalgithub.SendMetrics(ctx, client, pr)
		}
	}

	fmt.Println("Finish Sending Message")
}