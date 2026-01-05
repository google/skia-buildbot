package issuetracker

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"

	issuetracker "go.skia.org/infra/go/issuetracker/v1"
	regMocks "go.skia.org/infra/perf/go/regression/mocks"
	pb "go.skia.org/infra/perf/go/subscription/proto/v1"
)

func createIssueTrackerForTest(t *testing.T) (*issueTrackerImpl, *regMocks.Store, *httptest.Server) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := &issuetracker.Issue{
			IssueId: 12345,
		}
		err := json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	}))

	client, err := issuetracker.NewService(context.Background(), option.WithEndpoint(ts.URL), option.WithoutAuthentication())
	require.NoError(t, err)

	regStore := &regMocks.Store{}
	return &issueTrackerImpl{
		client:                client,
		FetchAnomaliesFromSql: true,
		regStore:              regStore,
	}, regStore, ts
}

func TestFileBug_Success(t *testing.T) {
	s, regStore, ts := createIssueTrackerForTest(t)
	defer ts.Close()

	regStore.On("GetSubscriptionsForRegressions", mock.Anything, mock.AnythingOfType("[]string")).Return(nil, nil, []*pb.Subscription{
		{
			BugComponent: "1235",
		},
	}, nil)

	req := &FileBugRequest{
		Title:       "Test Bug",
		Description: "This is a test bug.",
		Component:   "1234",
		Assignee:    "test@google.com",
		Ccs:         []string{"test2@google.com"},
	}

	issueID, err := s.FileBug(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, 12345, issueID)
}

func TestFileBug_NilRequest(t *testing.T) {
	s, _, ts := createIssueTrackerForTest(t)
	defer ts.Close()
	_, err := s.FileBug(context.Background(), nil)
	require.Error(t, err)
}

func TestFileBug_InvalidComponent(t *testing.T) {
	s, regStore, ts := createIssueTrackerForTest(t)
	defer ts.Close()

	regStore.On("GetSubscriptionsForRegressions", mock.Anything, mock.AnythingOfType("[]string")).Return(nil, nil, []*pb.Subscription{
		{
			BugComponent: "-1",
		},
	}, nil)

	req := &FileBugRequest{
		Component: "invalid",
	}
	_, err := s.FileBug(context.Background(), req)
	require.Error(t, err)
}

func TestFileBug_APIError(t *testing.T) {
	// This test is meant to fail - we use a server that always fails, see below.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	c, err := issuetracker.NewService(context.Background(), option.WithEndpoint(ts.URL), option.WithoutAuthentication())
	require.NoError(t, err)

	regStore := &regMocks.Store{}
	s := &issueTrackerImpl{
		client:                c,
		FetchAnomaliesFromSql: true,
		regStore:              regStore,
	}

	regStore.On("GetSubscriptionsForRegressions", mock.Anything, mock.AnythingOfType("[]string")).Return(nil, nil, []*pb.Subscription{
		{
			BugComponent: "1325852",
		},
	}, nil)

	req := &FileBugRequest{
		Title:       "Test Bug",
		Description: "This is a test bug.",
		Assignee:    "test@google.com",
		Ccs:         []string{"test2@google.com"},
	}

	_, err = s.FileBug(context.Background(), req)
	require.Error(t, err)
}

func TestFileBug_RequestBody(t *testing.T) {
	var receivedReq issuetracker.Issue

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &receivedReq)
		require.NoError(t, err)

		w.Header().Set("Content-Type", "application/json")
		resp := &issuetracker.Issue{
			IssueId: 12345,
		}
		err = json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	}))
	defer ts.Close()

	c, err := issuetracker.NewService(context.Background(), option.WithEndpoint(ts.URL), option.WithoutAuthentication())
	require.NoError(t, err)

	regStore := &regMocks.Store{}
	s := &issueTrackerImpl{
		client:                c,
		FetchAnomaliesFromSql: true,
		regStore:              regStore,
	}

	regStore.On("GetSubscriptionsForRegressions", mock.Anything, mock.AnythingOfType("[]string")).Return(nil, nil, []*pb.Subscription{
		{
			BugComponent: "8765",
		},
	}, nil)

	req := &FileBugRequest{
		Title:       "Test Bug Title",
		Description: "Test Bug Description",
		Component:   "5678",
		Assignee:    "assignee@google.com",
		Ccs:         []string{"cc1@google.com", "cc2@google.com"},
	}

	_, err = s.FileBug(context.Background(), req)
	require.NoError(t, err)

	require.Equal(t, "Test Bug Title", receivedReq.IssueState.Title)
	require.Equal(t, "Test Bug Description", receivedReq.IssueComment.Comment)
	// TODO(b/454614028) Change it to regStore value once migration is done.
	defaultComponentId := int64(1325852)
	// Note that componentID is overriden by the default value
	require.Equal(t, defaultComponentId, receivedReq.IssueState.ComponentId)
	require.Equal(t, "assignee@google.com", receivedReq.IssueState.Assignee.EmailAddress)
	require.Len(t, receivedReq.IssueState.Ccs, 2)
	require.Equal(t, "cc1@google.com", receivedReq.IssueState.Ccs[0].EmailAddress)
	require.Equal(t, "cc2@google.com", receivedReq.IssueState.Ccs[1].EmailAddress)
}
