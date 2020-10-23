package github

// Accesses github issues API.

import (
	"context"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"go.skia.org/infra/bugs-central/go/bugs"
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
	maxGithubResults = 1000

	githubSource types.IssueSource = "Github"
)

type githubPriorityData map[string]types.StandardizedPriority

var (
	// Maps the priority label names into the standardized priorities.
	githubProjectToPriorityData map[string]githubPriorityData = map[string]githubPriorityData{
		"flutter/flutter": {
			// https://github.com/flutter/flutter/labels/P0
			"P0": types.PriorityP0,
			// https://github.com/flutter/flutter/labels/P1
			"P1": types.PriorityP1,
			// https://github.com/flutter/flutter/labels/P2
			"P2": types.PriorityP2,
			// https://github.com/flutter/flutter/labels/P3
			"P3": types.PriorityP3,
			// https://github.com/flutter/flutter/labels/P4
			"P4": types.PriorityP4,
			// https://github.com/flutter/flutter/labels/P5
			"P5": types.PriorityP5,
			// https://github.com/flutter/flutter/labels/P6
			"P6": types.PriorityP6,
		},
	}
)

// githubFramework implements bugs.BugFramework for github repos.
type githubFramework struct {
	githubClient *github_api.GitHub
	projectName  string
	openIssues   *bugs.OpenIssues
	queryConfig  *GithubQueryConfig
}

// GithubQueryConfig is the config that will be used when querying github API.
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

// New returns an instance of the github implementation of bugs.BugFramework.
func New(ctx context.Context, repoOwner, repoName, credPath string, openIssues *bugs.OpenIssues, queryConfig *GithubQueryConfig) (bugs.BugFramework, error) {
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

	return &githubFramework{
		githubClient: githubClient,
		projectName:  fmt.Sprintf("%s/%s", repoOwner, repoName),
		openIssues:   openIssues,
		queryConfig:  queryConfig,
	}, nil
}

// See documentation for bugs.Search interface.
func (gh *githubFramework) Search(ctx context.Context) ([]*types.Issue, *types.IssueCountsData, error) {
	githubIssues, err := gh.githubClient.GetIssues(gh.queryConfig.Open, gh.queryConfig.Labels, maxGithubResults)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "could not get github issues with %s labels", gh.queryConfig.Labels)
	}

	// Convert github issues into bug_framework's generic issues
	issues := []*types.Issue{}
	countsData := &types.IssueCountsData{}
	priorityData, ok := githubProjectToPriorityData[gh.projectName]
	if !ok {
		sklog.Errorf("Could not find GithubPriorityData for project %s", gh.projectName)
	}
	for _, gi := range githubIssues {
		owner := ""
		if gi.GetAssignee() != nil {
			owner = gi.GetAssignee().GetEmail()
		}
		id := strconv.Itoa(gi.GetNumber())

		if gh.queryConfig.ExcludeLabels != nil {
			// Github API does not support a way to exclude labels right now. Going to do it
			// by looping and manually excluding issues.
			foundLabelToExclude := false
			for _, l := range gi.Labels {
				if util.In(l.GetName(), gh.queryConfig.ExcludeLabels) {
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
		if gh.queryConfig.PriorityRequired && priority == "" {
			continue
		}

		// Populate counts data.
		if owner == "" {
			countsData.UnassignedCount++
		}
		countsData.OpenCount++
		countsData.IncPriority(priority)
		countsData.CalculateSLOViolations(time.Now(), gi.GetCreatedAt(), gi.GetUpdatedAt(), priority)

		issues = append(issues, &types.Issue{
			Id:       id,
			State:    gi.GetState(),
			Priority: priority,
			Owner:    owner,
			Link:     gh.GetIssueLink("", id),

			CreatedTime:  gi.GetCreatedAt(),
			ModifiedTime: gi.GetUpdatedAt(),

			Title:   gi.GetTitle(),
			Summary: gi.GetBody(),
		})
	}

	return issues, countsData, nil
}

// See documentation for bugs.SearchClientAndPersist interface.
func (gh *githubFramework) SearchClientAndPersist(ctx context.Context, dbClient *db.FirestoreDB, runId string) error {
	issues, countsData, err := gh.Search(ctx)
	if err != nil {
		return skerr.Wrapf(err, "error when searching github")
	}
	sklog.Infof("%s Github issues %+v", gh.queryConfig.Client, countsData)

	// Construct the query description from labels and exclude labels.
	labelsInQuery := []string{}
	for _, l := range gh.queryConfig.Labels {
		labelsInQuery = append(labelsInQuery, fmt.Sprintf("label:\"%s\"", l))
	}
	queryDesc := strings.Join(labelsInQuery, "+")
	if gh.queryConfig.ExcludeLabels != nil {
		excludeLabels := []string{}
		for _, e := range gh.queryConfig.ExcludeLabels {
			excludeLabels = append(excludeLabels, fmt.Sprintf("-label:\"%s\"", e))
		}
		queryDesc = fmt.Sprintf("%s+%s", queryDesc, strings.Join(excludeLabels, "+"))
	}
	queryLink := fmt.Sprintf("https://github.com/%s/issues?q=is:issue+is:open+%s", gh.projectName, queryDesc)
	if gh.queryConfig.PriorityRequired {
		// The github query link and API does not support filtering by priority, it was manually done.
		// Show that priority was required in the query description.
		queryDesc += " priority-required"
	}
	countsData.QueryLink = queryLink
	// Github does not have an untriaged query link yet so use the open issues link instead.
	countsData.UntriagedQueryLink = queryLink
	client := gh.queryConfig.Client

	// Put in DB.
	if err := dbClient.PutInDB(ctx, client, githubSource, queryDesc, runId, countsData); err != nil {
		return skerr.Wrapf(err, "error putting github results in DB")
	}
	// Put in memory.
	gh.openIssues.PutOpenIssues(client, githubSource, queryDesc, issues)
	return nil
}

// See documentation for bugs.GetIssueLink interface.
func (gh *githubFramework) GetIssueLink(_, id string) string {
	return gh.githubClient.GetIssueUrlBase() + id
}
