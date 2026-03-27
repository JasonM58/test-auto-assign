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
	Github *github.Client
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
	Login string
	Score int
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
	fmt.Println("=== HANDLE PR EVENT CALLED ===")
	// =====================
	// 1. Load Event
	// =====================
	event, err := LoadPREvent()
	if err != nil {
		return err
	}

	fmt.Println("=== EVENT DEBUG ===")
	fmt.Println("Action:", event.Action)
	fmt.Println("PR:", event.PullRequest.Number)
	fmt.Println("Author:", event.PullRequest.User.Login)

	// Only handle PR opened
	if event.Action != "opened" {
		fmt.Println("Skip event:", event.Action)
		return nil
	}

	owner := event.PullRequest.Base.Repo.Owner.Login
	repo := event.PullRequest.Base.Repo.Name

	fmt.Println("Repo:", owner+"/"+repo)

	// =====================
	// 2. Get Contributors
	// =====================
	contributors, _, err := s.Github.Repositories.ListContributors(ctx, owner, repo, nil)
	if err != nil {
		return err
	}

	fmt.Println("=== CONTRIBUTORS ===")
	fmt.Println("Total contributors:", len(contributors))
	for _, c := range contributors {
		fmt.Println("User:", c.GetLogin(), "Contributions:", c.GetContributions())
	}

	// =====================
	// 3. Build Candidates
	// =====================
	var candidates []Candidate

	for _, c := range contributors {
		login := c.GetLogin()

		// filter invalid
		if login == "" ||
			login == event.PullRequest.User.Login ||
			strings.Contains(login, "bot") {
			continue
		}

		candidates = append(candidates, Candidate{
			Login: login,
			Score: c.GetContributions(),
		})
	}

	fmt.Println("=== FILTERED CANDIDATES ===")
	fmt.Println("Total candidates:", len(candidates))

	// =====================
	// 4. Edge Case
	// =====================
	if len(candidates) == 0 {
		fmt.Println("No candidates available")
		return nil
	}

	// =====================
	// 5. Sort Candidates
	// =====================
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	fmt.Println("=== RANKED CANDIDATES ===")
	for i, c := range candidates {
		fmt.Printf("%d. %s (score: %d)\n", i+1, c.Login, c.Score)
	}

	// =====================
	// 6. Random Selection (Top N)
	// =====================
	rand.Seed(time.Now().UnixNano())

	topN := 3
	if len(candidates) < topN {
		topN = len(candidates)
	}

	fmt.Println("=== TOP CANDIDATES ===")
	for i := 0; i < topN; i++ {
		fmt.Printf("%d. %s (score: %d)\n", i+1, candidates[i].Login, candidates[i].Score)
	}

	reviewer := candidates[rand.Intn(topN)]

	fmt.Println("Selected reviewer:", reviewer.Login)

	// =====================
	// 7. Assign Reviewer
	// =====================
	fmt.Println("Assigning reviewer to PR...")

	req := github.ReviewersRequest{
		Reviewers: []string{reviewer.Login},
	}

	_, _, err = s.Github.PullRequests.RequestReviewers(
		ctx,
		owner,
		repo,
		event.PullRequest.Number,
		req,
	)

	if err != nil {
		fmt.Println("Failed to assign reviewer:", err)
		return nil // do not fail workflow
	}

	fmt.Println("Assigned reviewer:", reviewer.Login)

	return nil
}
