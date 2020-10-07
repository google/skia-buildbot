package bugs

// Accesses github issues API.

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"

	"go.skia.org/infra/bugs-central/go/db"
	"go.skia.org/infra/bugs-central/go/types"
	github_api "go.skia.org/infra/go/github"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
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

type GithubPriorityData map[string]types.StandardizedPriority

type Github struct {
	githubClient *github_api.GitHub
	projectName  string
}

type GithubQueryConfig struct {
	// Slice of labels to look for in Github issues.
	Labels []string
	// Slice of labels to exclude.
	ExcludeLabels []string
	// Return only open issues.
	Open bool
	// If an issues has no priority label then do not include it in results.
	PriorityRequired bool
	// Which client's issues we are looking for.
	Client types.RecognizedClient
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
		githubClient: githubClient,
		projectName:  fmt.Sprintf("%s/%s", repoOwner, repoName),
	}, nil
}

func (gh *Github) Search(ctx context.Context, config interface{}) ([]*Issue, error) {
	githubQueryConfig, ok := config.(GithubQueryConfig)
	if !ok {
		return nil, errors.New("config must be GithubQueryConfig")
	}

	githubIssues, err := gh.githubClient.GetIssues(githubQueryConfig.Open, githubQueryConfig.Labels, MAX_GITHUB_RESULTS)
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
		id := strconv.Itoa(gi.GetNumber())

		if githubQueryConfig.ExcludeLabels != nil {
			// Github API does not support a way to exclude labels right now. Going to do it
			// by looping and manually excluding issues.
			foundLabelToExclude := false
			for _, l := range gi.Labels {
				if util.In(l.GetName(), githubQueryConfig.ExcludeLabels) {
					foundLabelToExclude = true
					break
				}
			}
			if foundLabelToExclude {
				continue
			}
		}

		// Find priority.
		priority := types.StandardizedPriority("")
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
		if githubQueryConfig.PriorityRequired && priority == "" {
			continue
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

func (gh *Github) PutInDB(ctx context.Context, config interface{}, count int, dbClient *db.FirestoreDB) error {
	githubQueryConfig, ok := config.(GithubQueryConfig)
	if !ok {
		return errors.New("config must be GithubQueryConfig")
	}

	// Construct the query description from labels and exclude labels.
	labelsInQuery := []string{}
	for _, l := range githubQueryConfig.Labels {
		labelsInQuery = append(labelsInQuery, fmt.Sprintf("label:\"%s\"", l))
	}
	queryDesc := strings.Join(labelsInQuery, "+")
	if githubQueryConfig.ExcludeLabels != nil {
		excludeLabels := []string{}
		for _, e := range githubQueryConfig.ExcludeLabels {
			excludeLabels = append(excludeLabels, fmt.Sprintf("-label:\"%s\"", e))
		}
		queryDesc = fmt.Sprintf("%s+%s", queryDesc, strings.Join(excludeLabels, "+"))
	}

	queryLink := fmt.Sprintf("https://github.com/%s/issues?q=is:issue+is:open%s", gh.projectName, queryDesc)
	if err := dbClient.PutInDB(ctx, githubQueryConfig.Client, GithubSource, queryDesc, queryLink, count); err != nil {
		return skerr.Wrapf(err, "error putting github results in DB")
	}
	return nil
}

func (gh *Github) GetLink(_, id string) string {
	return gh.githubClient.GetIssueUrlBase() + id
}
