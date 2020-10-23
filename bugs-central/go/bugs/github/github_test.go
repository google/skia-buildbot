package github

import (
	"context"
	"strconv"
	"testing"

	github_api "github.com/google/go-github/v29/github"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/bugs-central/go/bugs"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestGithubSearch(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()

	id1 := 11
	id2 := 22
	assignee := "superman@krypton.com"
	label1Name := "abc"
	label2Name := "xyz"
	label3Name := "123"
	label1 := github_api.Label{Name: &label1Name}
	label2 := github_api.Label{Name: &label2Name}
	label3 := github_api.Label{Name: &label3Name}
	issue1 := github_api.Issue{
		Number: &id1,
		Labels: []github_api.Label{label1, label2},
		Assignee: &github_api.User{
			Email: &assignee,
		},
	}
	issue2 := github_api.Issue{
		Number: &id2,
		Labels: []github_api.Label{label3},
		Assignee: &github_api.User{
			Email: &assignee,
		},
	}
	respBody := []byte(testutils.MarshalJSON(t, []*github_api.Issue{&issue1, &issue2}))
	r := mux.NewRouter()
	md := mockhttpclient.MockGetDialogue(respBody)
	r.Schemes("https").Host("api.github.com").Methods("GET").Path("/repos/kryptonians/krypton/issues").Queries("labels", "abc,xyz", "per_page", "1000", "state", "open").Handler(md)
	httpClient := mockhttpclient.NewMuxClient(r)

	githubClient, err := github.NewGitHub(ctx, "kryptonians", "krypton", httpClient)
	require.NoError(t, err)

	qc := &GithubQueryConfig{
		Labels:           []string{label1Name, label2Name},
		ExcludeLabels:    []string{label3Name},
		Open:             true,
		PriorityRequired: false,
		Client:           "Flutter-native",
	}
	g := githubFramework{
		githubClient: githubClient,
		projectName:  "flutter/flutter",
		openIssues:   bugs.InitOpenIssues(),
		queryConfig:  qc,
	}
	issues, countData, err := g.Search(ctx)
	require.NoError(t, err)

	// There should be one matching issue and one excluded issue.
	require.Equal(t, 1, len(issues))
	require.Equal(t, strconv.Itoa(id1), issues[0].Id)
	require.Equal(t, 1, countData.OpenCount)

	// Change the query config to have priority required. There should be
	// no matching issues because priority was not specified.
	qc.PriorityRequired = true
	issues, countData, err = g.Search(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, len(issues))
	require.Equal(t, 0, countData.OpenCount)
}
