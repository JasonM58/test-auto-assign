package autoassign

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

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

// =====================
// MAIN HANDLER
// =====================
func (s *AutoAssignService) HandlePREvent(ctx context.Context) error {
	fmt.Println("=== AUTO ASSIGN START ===")

	// =====================
	// VALIDATION
	// =====================
	if s.Github == nil {
		return fmt.Errorf("github client is nil")
	}
	if s.Metrics == nil {
		return fmt.Errorf("metrics provider is nil")
	}

	// =====================
	// LOAD EVENT
	// =====================
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

	// =====================
	// GET COLLABORATORS
	// =====================
	collaborators, _, err := s.Github.Repositories.ListCollaborators(ctx, owner, repo, nil)
	if err != nil {
		return fmt.Errorf("failed get collaborators: %w", err)
	}

	// =====================
	// GET PR DATA
	// =====================
	pr, _, err := s.Github.PullRequests.Get(ctx, owner, repo, prNumber)
	if err != nil {
		return fmt.Errorf("failed get PR: %w", err)
	}

	existing := map[string]bool{}
	for _, r := range pr.RequestedReviewers {
		existing[r.GetLogin()] = true
	}

	// =====================
	// BUILD CANDIDATES
	// =====================
	var candidates []Candidate

	for _, c := range collaborators {
		login := c.GetLogin()

		if login == "" ||
			login == prAuthor ||
			strings.Contains(login, "bot") ||
			existing[login] {
			continue
		}

		openPR, err := s.Metrics.GetOpenPRCount(ctx, owner, login)
		if err != nil {
			fmt.Printf("[WARN] openPR failed user=%s err=%v\n", login, err)
			continue // ⛔ skip user (lebih aman)
		}

		recent, err := s.Metrics.GetRecentReviewCount(ctx, owner, login)
		if err != nil {
			fmt.Printf("[WARN] recentReview failed user=%s err=%v\n", login, err)
			recent = 0 // fallback OK
		}

		score := CalculateScore(openPR, recent)

		fmt.Printf("[CANDIDATE] user=%s openPR=%d recent=%d score=%d\n",
			login, openPR, recent, score)

		candidates = append(candidates, Candidate{
			Login:         login,
			OpenPRCount:   openPR,
			RecentReviews: recent,
			Score:         score,
		})
	}

	// =====================
	// EDGE CASE
	// =====================
	if len(candidates) == 0 {
		fmt.Println("[INFO] No candidates available")
		return nil
	}

	// =====================
	// SORT
	// =====================
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	fmt.Println("=== RANKING ===")
	for i, c := range candidates {
		fmt.Printf("%d. %s (score=%d)\n", i+1, c.Login, c.Score)
	}

	// =====================
	// SELECT TOP N
	// =====================
	topN := 2
	if len(candidates) < topN {
		topN = len(candidates)
	}

	var reviewers []string
	for i := 0; i < topN; i++ {
		reviewers = append(reviewers, candidates[i].Login)
	}

	fmt.Println("=== FINAL RANKING ===")
	for i, c := range candidates {
		fmt.Printf("%d. %s (openPR=%d score=%d)\n",
			i+1, c.Login, c.OpenPRCount, c.Score)
	}

	fmt.Println("[SELECTED]", reviewers)

	// =====================
	// ASSIGN REVIEWER
	// =====================
	req := github.ReviewersRequest{
		Reviewers: reviewers,
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
		return nil // jangan fail workflow
	}

	fmt.Println("[ASSIGNED]", reviewers)

	return nil
}
