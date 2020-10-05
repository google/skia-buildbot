package bugs

// Accesses github issues API.

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"

	github_api "go.skia.org/infra/go/github"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/oauth2"
)

const (
	// Not clear what the maximum allowable results are for github API.
	MAX_GITHUB_RESULTS = 1000
)

var (
	// Maps the priority label names into the standardized priorities.
	GithubProjectToPriorityData map[string]GithubPriorityData = map[string]GithubPriorityData{
		"flutter/flutter": {
			// https://github.com/flutter/flutter/labels/P0
			"P0": PriorityP0,
			// https://github.com/flutter/flutter/labels/P1
			"P1": PriorityP1,
			// https://github.com/flutter/flutter/labels/P2
			"P2": PriorityP2,
			// https://github.com/flutter/flutter/labels/P3
			"P3": PriorityP3,
			// https://github.com/flutter/flutter/labels/P4
			"P4": PriorityP4,
			// https://github.com/flutter/flutter/labels/P5
			"P5": PriorityP5,
			// https://github.com/flutter/flutter/labels/P6
			"P6": PriorityP6,
		},
	}
)

type GithubPriorityData map[string]StandardizedPriority

type Github struct {
	client      *github_api.GitHub
	projectName string
}

type GithubQueryConfig struct {
	// Slice of labels to look for in Github issues.
	Labels []string
	// Return only open issues.
	Open bool
	// Return only unassigned issues.
	UnAssigned bool
}

func InitGithub(ctx context.Context, repoOwner, repoName, credPath string) (BugFramework, error) {
	gBody, err := ioutil.ReadFile(credPath)
	if err != nil {
		return nil, skerr.Wrapf(err, "could not find githubToken in %s", credPath)
	}
	gToken := strings.TrimSpace(string(gBody))
	githubTS := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: gToken})
	githubHttpClient := httputils.DefaultClientConfig().With2xxOnly().WithTokenSource(githubTS).Client()
	githubClient, err := github_api.NewGitHub(ctx, repoOwner, repoName, githubHttpClient)
	if err != nil {
		return nil, skerr.Wrapf(err, "could not instantiate github client")
	}

	return &Github{
		client:      githubClient,
		projectName: fmt.Sprintf("%s/%s", repoOwner, repoName),
	}, nil
}

func (gh *Github) Search(ctx context.Context, config interface{}) ([]*Issue, error) {
	githubQueryConfig, ok := config.(GithubQueryConfig)
	if !ok {
		return nil, errors.New("config must be GithubQueryConfig")
	}

	githubIssues, err := gh.client.GetIssues(githubQueryConfig.Open, githubQueryConfig.Labels, MAX_GITHUB_RESULTS)
	if err != nil {
		return nil, skerr.Wrapf(err, "could not get github issues with %s labels", githubQueryConfig.Labels)
	}

	// Convert github issues into bug_framework's generic issues
	issues := []*Issue{}
	priorityData, ok := GithubProjectToPriorityData[gh.projectName]
	if !ok {
		sklog.Errorf("Could not find GithubPriorityData for project %s", gh.projectName)
	}
	for _, gi := range githubIssues {
		owner := ""
		if gi.GetAssignee() != nil {
			owner = gi.GetAssignee().GetEmail()
		}
		if githubQueryConfig.UnAssigned && owner != "" {
			continue
		}
		id := strconv.Itoa(gi.GetNumber())

		// Find priority.
		priority := StandardizedPriority("")
		if priorityData != nil {
			// Go through labels for this issue to see if any of them are priority labels.
			for _, l := range gi.Labels {
				if p, ok := priorityData[*l.Name]; ok {
					priority = p
					// What happens if there are multiple priority labels attached? Use the
					// first one we encounter because that one *should* be the highest priority.
					break
				}
			}
		}

		issues = append(issues, &Issue{
			Id:       id,
			State:    gi.GetState(),
			Priority: priority,
			Owner:    owner,
			Link:     gh.GetLink("", id),

			CreatedTime:  gi.GetCreatedAt(),
			ModifiedTime: gi.GetUpdatedAt(),

			Title:   gi.GetTitle(),
			Summary: gi.GetBody(),
		})
	}

	return issues, nil
}

func (gh *Github) GetLink(_, id string) string {
	return gh.client.GetIssueUrlBase() + id
}
