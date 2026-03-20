package autoassign

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

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

	event, err := LoadPREvent()
	if err != nil {
		return err
	}

	fmt.Println("PR:", event.PullRequest.Number)
	fmt.Println("Author:", event.PullRequest.User.Login)

	owner := event.PullRequest.Base.Repo.Owner.Login
	repo := event.PullRequest.Base.Repo.Name

	fmt.Println("Repo:", owner+"/"+repo)

	// 1. Get contributors
	contributors, _, err := s.Github.Repositories.ListContributors(ctx, owner, repo, nil)
	if err != nil {
		return err
	}

	fmt.Println("=== CONTRIBUTORS ===")
	for _, c := range contributors {
		fmt.Println("User:", c.GetLogin())
	}

	// 2. Filter author
	var candidates []string

	for _, c := range contributors {
		login := c.GetLogin()

		if login == "" || login == event.PullRequest.User.Login {
			continue
		}

		candidates = append(candidates, login)
	}

	fmt.Println("=== CANDIDATES ===")
	for _, c := range candidates {
		fmt.Println("Candidate:", c)
	}

	// 3. Edge case
	if len(candidates) == 0 {
		return fmt.Errorf("no candidates available")
	}

	// 4. Assign reviewer
	reviewer := candidates[0]

	req := github.ReviewersRequest{
		Reviewers: []string{reviewer},
	}

	_, _, err = s.Github.PullRequests.RequestReviewers(
		ctx,
		owner,
		repo,
		event.PullRequest.Number,
		req,
	)

	if err != nil {
		return err
	}

	fmt.Println("Assigned reviewer:", reviewer)

	return nil
}
