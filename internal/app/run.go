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
	loc, _ := time.LoadLocation("Asia/Jakarta")
	today := time.Now().In(loc).Truncate(24 * time.Hour)
	tomorrow := today.Add(24 * time.Hour)

	client := internalgithub.SetupClient(ctx, cfg)
	shutdown := internaltelemetry.Setup(ctx, cfg)
	defer shutdown(ctx)

	notifier := internallark.NewNotifier(cfg, client)

	repos := internalgithub.FetchRepositories(ctx, client)
	for _, repo := range repos {
		fmt.Printf("🔍 Checking repo: %s/%s\n", repo.Owner, repo.Name)
		prs := internalgithub.FetchPRs(ctx, client, repo, today, tomorrow)
		sort.Slice(prs, func(i, j int) bool {
        return prs[i].IsOpen && !prs[j].IsOpen
    })
		for _, pr := range prs {
			if pr.IsOpen {
				notifier.NotifyOpenPR(ctx, pr)
			} else {
				notifier.NotifyMergedPR(ctx, pr)
				internalgithub.SendMetrics(ctx, client, pr)
			}
		}
	}
}

