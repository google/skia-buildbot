package monorail

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"go.skia.org/infra/bugs-central/go/bugs"
	"go.skia.org/infra/go/mockhttpclient"
	monorail_srv "go.skia.org/infra/go/monorail/v3"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestMonorailSearch(t *testing.T) {
	unittest.SmallTest(t)
	ctx := context.Background()

	mc := &MonorailQueryConfig{
		Instance: "skia",
		Query:    "test-query",
		Client:   "Skia",
	}
	reqBody := []byte(fmt.Sprintf(`{"projects": ["projects/%s"], "query": "%s", "page_token": ""}`, mc.Instance, mc.Query))
	issue1 := "123"
	issue2 := "456"

	respBody := []byte(fmt.Sprintf(`{"issues":[{"name": "%s"},{"name": "%s"}],"nextPageToken":""}`, issue1, issue2))
	// Monorail API prepends chars to prevent XSS.
	respBody = append([]byte("abcd\n"), respBody...)

	r := mux.NewRouter()
	md := mockhttpclient.MockPostDialogueWithResponseCode("application/json", reqBody, respBody, http.StatusOK)
	r.Schemes("https").Host("api-dot-monorail-prod.appspot.com").Methods("POST").Path("/prpc/monorail.v3.Issues/SearchIssues").Handler(md)
	httpClient := mockhttpclient.NewMuxClient(r)

	testToken := oauth2.Token{
		AccessToken: "access-token",
	}
	monorailService := &monorail_srv.MonorailService{
		Token:      &testToken,
		HttpClient: httpClient,
	}
	m := monorail{
		monorailService: monorailService,
		openIssues:      bugs.InitOpenIssues(),
		queryConfig:     mc,
	}

	issues, countsData, err := m.Search(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, len(issues))
	require.Equal(t, issue1, issues[0].Id)
	require.Equal(t, issue2, issues[1].Id)
	require.Equal(t, 2, countsData.OpenCount)
	require.Equal(t, 2, countsData.UnassignedCount)
	require.Equal(t, 0, countsData.UntriagedCount)

	// Set UnassignedIsUntriaged and assert.
	mc.UnassignedIsUntriaged = true
	issues, countsData, err = m.Search(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, len(issues))
	require.Equal(t, issue1, issues[0].Id)
	require.Equal(t, issue2, issues[1].Id)
	require.Equal(t, 2, countsData.OpenCount)
	require.Equal(t, 2, countsData.UnassignedCount)
	require.Equal(t, 2, countsData.UntriagedCount)
}
