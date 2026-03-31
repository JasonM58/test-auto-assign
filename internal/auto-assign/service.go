package autoassign

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/go-github/v61/github"
)

type AutoAssignService struct {
	Github  *github.Client
	Metrics MetricsProvider
}

type PREvent struct {
	Action string `json:"action"`

	PullRequest struct {
		Number int `json:"number"`

		User struct {
			Login string `json:"login"`
		} `json:"user"`

		Base struct {
			Repo struct {
				Name  string `json:"name"`
				Owner struct {
					Login string `json:"login"`
				} `json:"owner"`
			} `json:"repo"`
		} `json:"base"`
	} `json:"pull_request"`
}

type Candidate struct {
	Login         string
	Workload      int
	RecentReviews int
	Score         float64
}

// =====================
// LOAD EVENT
// =====================
func LoadPREvent() (*PREvent, error) {
	path := os.Getenv("GITHUB_EVENT_PATH")
	if path == "" {
		return nil, fmt.Errorf("GITHUB_EVENT_PATH not set")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed read event file: %w", err)
	}

	var event PREvent
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, fmt.Errorf("failed parse event: %w", err)
	}

	return &event, nil
}

func getOrDefault(m map[string]int, key string, fallback int) int {
	if v, ok := m[key]; ok {
		return v
	}
	return fallback
}

func normalize(value, max int) float64 {
	if max == 0 {
		return 0
	}
	return float64(value) / float64(max)
}

func CalculateScore(workload, recent, maxWorkload, maxRecent int) float64 {
	wNorm := normalize(workload, maxWorkload)
	rNorm := normalize(recent, maxRecent)

	const workloadWeight = 0.7
	const recentWeight = 0.3

	score := (1-wNorm)*workloadWeight + (1-rNorm)*recentWeight
	return score
}

func (s *AutoAssignService) HandlePREvent(ctx context.Context) error {
	fmt.Println("=== AUTO ASSIGN START ===")

	if s.Github == nil {
		return fmt.Errorf("github client is nil")
	}
	if s.Metrics == nil {
		return fmt.Errorf("metrics provider is nil")
	}

	event, err := LoadPREvent()
	if err != nil {
		return err
	}

	if event.Action != "opened" {
		fmt.Println("Skip event:", event.Action)
		return nil
	}

	owner := event.PullRequest.Base.Repo.Owner.Login
	repo := event.PullRequest.Base.Repo.Name
	prNumber := event.PullRequest.Number
	prAuthor := event.PullRequest.User.Login

	fmt.Printf("Repo: %s/%s\n", owner, repo)
	fmt.Printf("PR: %d | Author: %s\n", prNumber, prAuthor)

	//  Fetch Data
	collaborators, _, err := s.Github.Repositories.ListCollaborators(ctx, owner, repo, nil)
	if err != nil {
		return fmt.Errorf("failed get collaborators: %w", err)
	}

	pr, _, err := s.Github.PullRequests.Get(ctx, owner, repo, prNumber)
	if err != nil {
		return fmt.Errorf("failed get PR: %w", err)
	}

	existing := map[string]bool{}
	for _, r := range pr.RequestedReviewers {
		existing[r.GetLogin()] = true
	}

	// Fetch Metric
	workloads, err := s.Metrics.GetReviewWorkload(ctx)
	if err != nil {
		fmt.Printf("[WARN] workload fetch failed: %v\n", err)
		workloads = map[string]int{}
	}

	recents, err := s.Metrics.GetRecentReviewCount(ctx)
	if err != nil {
		fmt.Printf("[WARN] recent fetch failed: %v\n", err)
		recents = map[string]int{}
	}

	fmt.Printf("[METRICS] workloads=%v\n", workloads)
	fmt.Printf("[METRICS] recents=%v\n", recents)

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

	var candidates []Candidate

	for _, c := range collaborators {
		login := c.GetLogin()

		if login == "" ||
			login == prAuthor ||
			strings.Contains(strings.ToLower(login), "bot") ||
			existing[login] {
			continue
		}

		workload := getOrDefault(workloads, login, maxWorkload)
		recent := getOrDefault(recents, login, 0)

		score := CalculateScore(workload, recent, maxWorkload, maxRecent)

		fmt.Printf("[CANDIDATE] %s → workload=%d recent=%d score=%.4f\n",
			login, workload, recent, score)

		candidates = append(candidates, Candidate{
			Login:         login,
			Workload:      workload,
			RecentReviews: recent,
			Score:         score,
		})
	}

	if len(candidates) == 0 {
		fmt.Println("[INFO] No candidates available")
		return nil
	}

	// Sort + Fairness
	rand.Seed(time.Now().UnixNano())

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return rand.Intn(2) == 0
		}
		return candidates[i].Score > candidates[j].Score
	})

	fmt.Println("=== RANKING ===")
	for i, c := range candidates {
		fmt.Printf("%d. %s (score=%.4f)\n", i+1, c.Login, c.Score)
	}

	// Select Top N
	topN := 2
	if len(candidates) < topN {
		topN = len(candidates)
	}

	selected := make([]string, 0, topN)
	for i := 0; i < topN; i++ {
		selected = append(selected, candidates[i].Login)
	}

	fmt.Println("[SELECTED]", selected)

	// Assign Reviewer
	req := github.ReviewersRequest{
		Reviewers: selected,
	}

	_, _, err = s.Github.PullRequests.RequestReviewers(
		ctx,
		owner,
		repo,
		prNumber,
		req,
	)

	if err != nil {
		fmt.Printf("[ERROR] assign reviewer failed: %v\n", err)
		return nil // non-blocking
	}

	fmt.Println("[ASSIGNED]", selected)

	return nil
}
