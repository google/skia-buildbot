package monorail // import "go.skia.org/infra/go/monorail/v3"

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestGetEmail_Success(t *testing.T) {
	unittest.SmallTest(t)
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
	r := mux.NewRouter()
	md := mockhttpclient.MockPostDialogueWithResponseCode("application/json", reqBody, respBody, http.StatusOK)
	r.Schemes("https").Host("api-dot-monorail-prod.appspot.com").Methods("POST").Path("/prpc/monorail.v3.Users/GetUser").Handler(md)
	httpClient := mockhttpclient.NewMuxClient(r)

	ms := &MonorailService{
		HttpClient: httpClient,
	}
	monorailUser, err := ms.GetEmail(testUserName)
	require.NoError(t, err)
	require.Equal(t, testUserEmail, monorailUser.DisplayName)
}

func TestSetOwnerAndAddComment_Success(t *testing.T) {
	unittest.SmallTest(t)
	testInstance := "test-project"
	testOwner := "superman@krypton.com"
	testComment := "test comment"
	testId := "1000"

	// Mock request and response.
	reqBody := []byte(fmt.Sprintf(`{"deltas": [{"issue": {"name": "projects/%s/issues/%s", "owner": {"user": "users/%s"}}, "update_mask": "owner"}], "comment_content": "%s", "notify_type": "EMAIL"}`, testInstance, testId, testOwner, testComment))
	// // Monorail API prepends chars to prevent XSS.
	respBody := []byte("abcd\n{}")

	// Mock HTTP client.
	r := mux.NewRouter()
	md := mockhttpclient.MockPostDialogueWithResponseCode("application/json", reqBody, respBody, http.StatusOK)
	r.Schemes("https").Host("api-dot-monorail-prod.appspot.com").Methods("POST").Path("/prpc/monorail.v3.Issues/ModifyIssues").Handler(md)
	httpClient := mockhttpclient.NewMuxClient(r)

	ms := &MonorailService{
		HttpClient: httpClient,
	}
	err := ms.SetOwnerAndAddComment(testInstance, testOwner, testComment, testId)
	require.NoError(t, err)
}

func TestGetIssue_Success(t *testing.T) {
	unittest.SmallTest(t)
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
	r := mux.NewRouter()
	md := mockhttpclient.MockPostDialogueWithResponseCode("application/json", reqBody, respBody, http.StatusOK)
	r.Schemes("https").Host("api-dot-monorail-prod.appspot.com").Methods("POST").Path("/prpc/monorail.v3.Issues/GetIssue").Handler(md)
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
	unittest.SmallTest(t)
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
	reqBody := []byte(fmt.Sprintf(`{"parent": "projects/%s", "issue": {"owner": {"user": "users/%s"}, "status": {"status": "%s"}, "summary": "%s", "labels": [{"label": "%s"}], "components": [{"component": "projects/%s/componentDefs/%s"}], "cc_users": [{"user": "users/%s"}], "field_values": [{"field": "%s", "value": "%s"}, {"field": "%s", "value": "%s"}]}, "description": "%s"}`, instance, testOwner, testStatus, testSummary, testLabelName, instance, testComponentDefID, testCCUser, testPriorityField, testPriorityValue, testIssueTypeField, testIssueTypeValue, testDescription))
	respBody, err := json.Marshal(&MonorailIssue{
		Title: testSummary,
	})
	// Monorail API prepends chars to prevent XSS.
	respBody = append([]byte("abcd\n"), respBody...)
	require.NoError(t, err)

	// Mock HTTP client.
	r := mux.NewRouter()
	md := mockhttpclient.MockPostDialogueWithResponseCode("application/json", reqBody, respBody, http.StatusOK)
	r.Schemes("https").Host("api-dot-monorail-prod.appspot.com").Methods("POST").Path("/prpc/monorail.v3.Issues/MakeIssue").Handler(md)
	httpClient := mockhttpclient.NewMuxClient(r)

	// Full E2E run.
	ms := &MonorailService{
		HttpClient: httpClient,
	}
	issue, err := ms.MakeIssue(instance, testOwner, testSummary, testDescription, testStatus, testPriorityValue, testIssueTypeValue, []string{testLabelName}, []string{testComponentDefID}, []string{testCCUser})
	require.NoError(t, err)
	require.Equal(t, testSummary, issue.Title)

	// Using an unsupported project should return an error for failing to find
	// it's priority field name and type field name.
	instance = "unsupported-project-name"
	issue, err = ms.MakeIssue(instance, testOwner, testSummary, testDescription, testStatus, testPriorityValue, testIssueTypeValue, []string{testLabelName}, []string{testComponentDefID}, []string{testCCUser})
	require.Error(t, err)
	require.Nil(t, issue)
}

func TestSearchIssuesWithPagination_Success(t *testing.T) {
	unittest.SmallTest(t)
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
	r := mux.NewRouter()
	md := mockhttpclient.MockPostDialogueWithResponseCode("application/json", reqBody, respBody, http.StatusOK)
	r.Schemes("https").Host("api-dot-monorail-prod.appspot.com").Methods("POST").Path("/prpc/monorail.v3.Issues/SearchIssues").Handler(md)
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
