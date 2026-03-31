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

	metricsProvider := internalautoassign.NewPrometheusMetrics(cfg.GetPrometheusURL(), nil)

	// Wire optional production-tuning from config (zero values keep defaults).
	if v := cfg.GetPrometheusQueryTimeoutSecs(); v > 0 {
		metricsProvider.QueryTimeout = time.Duration(v) * time.Second
	}
	if v := cfg.GetPrometheusMaxRetries(); v > 0 {
		metricsProvider.MaxRetries = v
	}
	if v := cfg.GetPrometheusRetryBaseDelayMs(); v > 0 {
		metricsProvider.RetryBaseDelay = time.Duration(v) * time.Millisecond
	}

	// Startup health check — fail fast if VictoriaMetrics is unreachable.
	if cfg.GetPrometheusURL() != "" {
		if err := metricsProvider.Ping(ctx); err != nil {
			log.Printf("⚠️ Prometheus/VictoriaMetrics health check failed: %v (continuing anyway)", err)
		} else {
			log.Println("✅ Prometheus/VictoriaMetrics is reachable")
		}
	}

	org := "ionextai"

	// =========================
	// MODE: METRICS ONLY (DEBUG)
	// =========================
	if mode == "metrics" {
		fmt.Println("=== METRICS MODE ===")

		workloads, err := metricsProvider.GetReviewWorkload(ctx)
		if err != nil {
			fmt.Printf("❌ workload fetch failed: %v\n", err)
			workloads = map[string]int{}
		}

		recents, err := metricsProvider.GetRecentReviewCount(ctx)
		if err != nil {
			fmt.Printf("❌ recent fetch failed: %v\n", err)
			recents = map[string]int{}
		}

		// compute max values
		maxWorkload := 1
		for _, v := range workloads {
			if v > maxWorkload {
				maxWorkload = v
			}
		}

		maxRecent := 1
		for _, v := range recents {
			if v > maxRecent {
				maxRecent = v
			}
		}

		// print all reviewer metrics
		seen := map[string]bool{}
		for user := range workloads {
			seen[user] = true
		}
		for user := range recents {
			seen[user] = true
		}

		for user := range seen {
			w := workloads[user]
			r := recents[user]
			score := internalautoassign.CalculateScore(w, r, maxWorkload, maxRecent)
			fmt.Printf("User=%s Workload=%d Recent=%d Score=%.4f\n", user, w, r, score)
		}

		return
	}

	if mode == "autoassign" {
		fmt.Println("=== AUTO ASSIGN MODE ===")

		autoAssignService := internalautoassign.AutoAssignService{
			Github:  client,
			Metrics: metricsProvider,
		}

		if err := autoAssignService.HandlePREvent(ctx); err != nil {
			log.Printf("Auto assign failed: %v", err)
		}

		// Exit quickly so the GitHub Action doesn't hang processing telemetry!
		return
	}

	fmt.Println("=== TELEMETRY MODE ===")

	shutdown := internaltelemetry.Setup(ctx, cfg)
	defer shutdown(ctx)

	notifier := internallark.NewNotifier(cfg, client)

	//Fetch PR
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

	//Process PR
	for _, pr := range prs {

		metrics, err := githubclient.GetPRMetrics(
			ctx,
			client,
			pr.Repo.Owner,
			pr.Repo.Name,
			pr.RawPR,
		)
		if err != nil {
			log.Printf("❌ Failed metrics PR #%d: %v", pr.Number, err)
			continue
		}

		if err := githubclient.SendBasePRMetrics(ctx, metrics, prCounts); err != nil {
			log.Printf("❌ Failed base metrics PR #%d: %v", pr.Number, err)
		}

		if pr.IsOpen {
			notifier.NotifyOpenPR(ctx, pr)

			if err := githubclient.SendWorkloadMetrics(ctx, metrics); err != nil {
				log.Printf("❌ Failed workload PR #%d: %v", pr.Number, err)
			}

		} else {

			if pr.RawPR.GetMergedAt().IsZero() {
				continue // skip closed but not merged
			}

			notifier.NotifyMergedPR(ctx, pr)

			if err := githubclient.SendHistoricalMetrics(ctx, metrics); err != nil {
				log.Printf("❌ Failed historical PR #%d: %v", pr.Number, err)
			}
		}
	}
}
