package monorail // import "go.skia.org/infra/go/monorail/v3"

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/mockhttpclient"
)

func TestGetEmail_Success(t *testing.T) {
	testUserName := "superman"
	testUserEmail := "superman@krypton.com"

	// Mock request and response.
	reqBody := []byte(fmt.Sprintf(`{"name": "%s"}`, testUserName))
	respBody, err := json.Marshal(&MonorailUser{
		DisplayName: testUserEmail,
	})
	// Monorail API prepends chars to prevent XSS.
	respBody = append([]byte("abcd\n"), respBody...)
	require.NoError(t, err)

	// Mock HTTP client.
	r := chi.NewRouter()
	md := mockhttpclient.MockPostDialogueWithResponseCode("application/json", reqBody, respBody, http.StatusOK)
	r.With(
		mockhttpclient.SchemeMatcher("https"),
		mockhttpclient.HostMatcher("api-dot-monorail-prod.appspot.com")).
		Post("/prpc/monorail.v3.Users/GetUser", md.ServeHTTP)
	httpClient := mockhttpclient.NewMuxClient(r)

	ms := &MonorailService{
		HttpClient: httpClient,
	}
	monorailUser, err := ms.GetEmail(testUserName)
	require.NoError(t, err)
	require.Equal(t, testUserEmail, monorailUser.DisplayName)
}

func TestSetOwnerAndAddComment_Success(t *testing.T) {
	testInstance := "test-project"
	testOwner := "superman@krypton.com"
	testComment := "test comment"
	testId := "1000"

	// Mock request and response.
	reqBody := []byte(fmt.Sprintf(`{"deltas": [{"issue": {"name": "projects/%s/issues/%s", "owner": {"user": "users/%s"}}, "update_mask": "owner"}], "comment_content": "%s", "notify_type": "EMAIL"}`, testInstance, testId, testOwner, testComment))
	// // Monorail API prepends chars to prevent XSS.
	respBody := []byte("abcd\n{}")

	// Mock HTTP client.
	r := chi.NewRouter()
	md := mockhttpclient.MockPostDialogueWithResponseCode("application/json", reqBody, respBody, http.StatusOK)
	r.With(
		mockhttpclient.SchemeMatcher("https"),
		mockhttpclient.HostMatcher("api-dot-monorail-prod.appspot.com")).
		Post("/prpc/monorail.v3.Issues/ModifyIssues", md.ServeHTTP)
	httpClient := mockhttpclient.NewMuxClient(r)

	ms := &MonorailService{
		HttpClient: httpClient,
	}
	err := ms.SetOwnerAndAddComment(testInstance, testOwner, testComment, testId)
	require.NoError(t, err)
}

func TestGetIssue_Success(t *testing.T) {
	testIssueName := "projects/test-project/issues/10000"
	testTitle := "Test Title."

	// Mock request and response.
	reqBody := []byte(fmt.Sprintf(`{"name": "%s"}`, testIssueName))
	respBody, err := json.Marshal(&MonorailIssue{
		Name:  testIssueName,
		Title: testTitle,
	})
	// Monorail API prepends chars to prevent XSS.
	respBody = append([]byte("abcd\n"), respBody...)
	require.NoError(t, err)

	// Mock HTTP client.
	r := chi.NewRouter()
	md := mockhttpclient.MockPostDialogueWithResponseCode("application/json", reqBody, respBody, http.StatusOK)
	r.With(
		mockhttpclient.SchemeMatcher("https"),
		mockhttpclient.HostMatcher("api-dot-monorail-prod.appspot.com")).
		Post("/prpc/monorail.v3.Issues/GetIssue", md.ServeHTTP)
	httpClient := mockhttpclient.NewMuxClient(r)

	ms := &MonorailService{
		HttpClient: httpClient,
	}
	issue, err := ms.GetIssue(testIssueName)
	require.NoError(t, err)
	require.Equal(t, testIssueName, issue.Name)
	require.Equal(t, testTitle, issue.Title)
}

func TestMakeIssue_Success(t *testing.T) {
	instance := skiaMonorailInstance
	testOwner := "superman@krypton.com"
	testSummary := "Test Issue Summary"
	testDescription := "Test Issue Description"
	testStatus := "Open"

	testPriorityField := ProjectToPriorityFieldNames[instance]
	testPriorityValue := "P1"
	testIssueTypeField := ProjectToTypeFieldNames[instance]
	testIssueTypeValue := "Task"
	testLabelName := "Test-Label1"
	testComponentDefID := "2000"
	testCCUser := "batman@gotham.com"

	// Mock request and response.
	reqBody := []byte(fmt.Sprintf(`{"parent": "projects/%s", "issue": {"owner": {"user": "users/%s"}, "status": {"status": "%s"}, "summary": "%s", "labels": [{"label": "%s"}], "components": [{"component": "projects/%s/componentDefs/%s"}], "cc_users": [{"user": "users/%s"}], "field_values": [{"field": "%s", "value": "%s"},{"field": "%s", "value": "%s"}]}, "description": "%s"}`, instance, testOwner, testStatus, testSummary, testLabelName, instance, testComponentDefID, testCCUser, testPriorityField, testPriorityValue, testIssueTypeField, testIssueTypeValue, testDescription))
	respBody, err := json.Marshal(&MonorailIssue{
		Title: testSummary,
	})
	// Monorail API prepends chars to prevent XSS.
	respBody = append([]byte("abcd\n"), respBody...)
	require.NoError(t, err)

	// Mock HTTP client.
	r := chi.NewRouter()
	md := mockhttpclient.MockPostDialogueWithResponseCode("application/json", reqBody, respBody, http.StatusOK)
	r.With(
		mockhttpclient.SchemeMatcher("https"),
		mockhttpclient.HostMatcher("api-dot-monorail-prod.appspot.com")).
		Post("/prpc/monorail.v3.Issues/MakeIssue", md.ServeHTTP)
	httpClient := mockhttpclient.NewMuxClient(r)

	// Full E2E run.
	ms := &MonorailService{
		HttpClient: httpClient,
	}
	issue, err := ms.MakeIssue(instance, testOwner, testSummary, testDescription, testStatus, testPriorityValue, testIssueTypeValue, []string{testLabelName}, []string{testComponentDefID}, []string{testCCUser})
	require.NoError(t, err)
	require.Equal(t, testSummary, issue.Title)
}

func TestMakeIssue_NoPriorityNoType_Success(t *testing.T) {
	const instance = "unrecognized-project-name"
	const testOwner = "superman@krypton.com"
	const testSummary = "Test Issue Summary"
	const testDescription = "Test Issue Description"
	const testStatus = "Open"

	const testPriorityValue = "P1"
	const testIssueTypeValue = "Task"
	const testLabelName = "Test-Label1"
	const testComponentDefID = "2000"
	const testCCUser = "batman@gotham.com"

	// Mock request and response.
	reqBody := []byte(fmt.Sprintf(`{"parent": "projects/%s", "issue": {"owner": {"user": "users/%s"}, "status": {"status": "%s"}, "summary": "%s", "labels": [{"label": "%s"},{"label": "%s"},{"label": "%s"}], "components": [{"component": "projects/%s/componentDefs/%s"}], "cc_users": [{"user": "users/%s"}], "field_values": []}, "description": "%s"}`, instance, testOwner, testStatus, testSummary, testLabelName, testPriorityValue, testIssueTypeValue, instance, testComponentDefID, testCCUser, testDescription))
	respBody, err := json.Marshal(&MonorailIssue{
		Title: testSummary,
	})
	// Monorail API prepends chars to prevent XSS.
	respBody = append([]byte("abcd\n"), respBody...)
	require.NoError(t, err)

	// Mock HTTP client.
	r := chi.NewRouter()
	md := mockhttpclient.MockPostDialogueWithResponseCode("application/json", reqBody, respBody, http.StatusOK)
	r.With(
		mockhttpclient.SchemeMatcher("https"),
		mockhttpclient.HostMatcher("api-dot-monorail-prod.appspot.com")).
		Post("/prpc/monorail.v3.Issues/MakeIssue", md.ServeHTTP)
	httpClient := mockhttpclient.NewMuxClient(r)

	// Full E2E run.
	ms := &MonorailService{
		HttpClient: httpClient,
	}
	issue, err := ms.MakeIssue(instance, testOwner, testSummary, testDescription, testStatus, testPriorityValue, testIssueTypeValue, []string{testLabelName}, []string{testComponentDefID}, []string{testCCUser})
	require.NoError(t, err)
	require.Equal(t, testSummary, issue.Title)
}

func TestSearchIssuesWithPagination_Success(t *testing.T) {
	testInstance := "test-project"
	testQuery := "test-query"
	testIssue1 := "123"
	testIssue2 := "456"

	// Mock request and response.
	reqBody := []byte(fmt.Sprintf(`{"projects": ["projects/%s"], "query": "%s", "page_token": ""}`, testInstance, testQuery))
	respBody := []byte(fmt.Sprintf(`{"issues":[{"name": "%s"},{"name": "%s"}],"nextPageToken":""}`, testIssue1, testIssue2))
	// Monorail API prepends chars to prevent XSS.
	respBody = append([]byte("abcd\n"), respBody...)

	// Mock HTTP client.
	r := chi.NewRouter()
	md := mockhttpclient.MockPostDialogueWithResponseCode("application/json", reqBody, respBody, http.StatusOK)
	r.With(
		mockhttpclient.SchemeMatcher("https"),
		mockhttpclient.HostMatcher("api-dot-monorail-prod.appspot.com")).
		Post("/prpc/monorail.v3.Issues/SearchIssues", md.ServeHTTP)
	httpClient := mockhttpclient.NewMuxClient(r)

	ms := &MonorailService{
		HttpClient: httpClient,
	}
	issues, err := ms.SearchIssuesWithPagination(testInstance, testQuery)
	require.NoError(t, err)
	require.Len(t, issues, 2)
	require.Equal(t, testIssue1, issues[0].Name)
	require.Equal(t, testIssue2, issues[1].Name)
}
