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

func LoadPREvent() (*PREvent, error) {
	path := os.Getenv("GITHUB_EVENT_PATH")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var event PREvent
	err = json.Unmarshal(data, &event)
	if err != nil {
		return nil, err
	}

	return &event, nil
}

func (s *AutoAssignService) HandlePREvent(ctx context.Context) error {
	fmt.Println("=== AUTO ASSIGN START ===")

	// =====================
	// 1. Load Event
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

	fmt.Println("Repo:", owner+"/"+repo)
	fmt.Println("PR:", prNumber)

	// =====================
	// 2. Get Collaborators
	// =====================
	collaborators, _, err := s.Github.Repositories.ListCollaborators(ctx, owner, repo, nil)
	if err != nil {
		return err
	}

	// =====================
	// 3. Get Existing Reviewers
	// =====================
	pr, _, err := s.Github.PullRequests.Get(ctx, owner, repo, prNumber)
	if err != nil {
		return err
	}

	existing := map[string]bool{}
	for _, r := range pr.RequestedReviewers {
		existing[r.GetLogin()] = true
	}

	// =====================
	// 4. Build Candidates
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
			fmt.Println("error openPR:", login, err)
			openPR = 0
		}

		recent, err := s.Metrics.GetRecentReviewCount(ctx, owner, login)
		if err != nil {
			fmt.Println("error recent:", login, err)
			recent = 0
		}

		score := CalculateScore(openPR, recent)

		fmt.Printf("User=%s OpenPR=%d Score=%d\n", login, openPR, score)

		candidates = append(candidates, Candidate{
			Login:         login,
			OpenPRCount:   openPR,
			RecentReviews: recent,
			Score:         score,
		})
	}

	// =====================
	// 5. Edge Case
	// =====================
	if len(candidates) == 0 {
		fmt.Println("No candidates available")
		return nil
	}

	// =====================
	// 6. Sort by Score
	// =====================
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	fmt.Println("=== RANKING ===")
	for i, c := range candidates {
		fmt.Printf("%d. %s (score=%d)\n", i+1, c.Login, c.Score)
	}

	// =====================
	// 7. Select Top 2
	// =====================
	topN := 2
	if len(candidates) < topN {
		topN = len(candidates)
	}

	var reviewers []string
	for i := 0; i < topN; i++ {
		reviewers = append(reviewers, candidates[i].Login)
	}

	fmt.Println("Selected reviewers:", reviewers)

	// =====================
	// 8. Assign Reviewer
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
		fmt.Println("Failed to assign reviewer:", err)
		return nil
	}

	fmt.Println("Assigned reviewers:", reviewers)

	return nil
}
