package issuetracker

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"

	issuetracker "go.skia.org/infra/go/issuetracker/v1"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/regression"
	regMocks "go.skia.org/infra/perf/go/regression/mocks"
	pb "go.skia.org/infra/perf/go/subscription/proto/v1"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
)

var sampleParamsetMap = map[string]string{
	"bot":         "botxyz",
	"benchmark":   "benchmark",
	"story":       "story",
	"measurement": "measurement",
	"stat":        "stat",
}

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

func createIssueTrackerForTestInterceptRequests(t *testing.T) (*issueTrackerImpl, *regMocks.Store, *httptest.Server, *issuetracker.Issue, *issuetracker.IssueComment) {
	var receivedReq issuetracker.Issue
	var receivedCommentReq issuetracker.IssueComment
	var counter int

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		if counter == 0 {
			err = json.Unmarshal(body, &receivedReq)
		} else {
			err = json.Unmarshal(body, &receivedCommentReq)
		}
		counter += 1
		require.NoError(t, err)

		w.Header().Set("Content-Type", "application/json")
		resp := &issuetracker.Issue{
			IssueId: 12345,
		}
		err = json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	}))

	c, err := issuetracker.NewService(context.Background(), option.WithEndpoint(ts.URL), option.WithoutAuthentication())
	require.NoError(t, err)

	regStore := &regMocks.Store{}
	s := &issueTrackerImpl{
		client:                c,
		FetchAnomaliesFromSql: true,
		regStore:              regStore,
		urlBase:               "http://test.com",
	}
	return s, regStore, ts, &receivedReq, &receivedCommentReq
}

func TestFileBug_Success(t *testing.T) {
	s, regStore, ts := createIssueTrackerForTest(t)
	defer ts.Close()

	regStore.On("GetSubscriptionsForRegressions", mock.Anything, mock.AnythingOfType("[]string")).Return([]string{"1"}, []int64{1}, []*pb.Subscription{
		{
			BugComponent: "1235",
		},
	}, nil)
	regStore.On("GetByIDs", mock.Anything, mock.AnythingOfType("[]string")).Return([]*regression.Regression{}, nil)

	req := &FileBugRequest{
		Title:       "Test Bug",
		Description: "This is a test bug.",
		Component:   "1234",
		Assignee:    "test@google.com",
		Ccs:         []string{"test2@google.com"},
		Keys:        []string{"1"},
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

	regStore.On("GetSubscriptionsForRegressions", mock.Anything, mock.AnythingOfType("[]string")).Return([]string{"1"}, []int64{1}, []*pb.Subscription{
		{
			BugComponent: "invalid",
		},
	}, nil)
	regStore.On("GetByIDs", mock.Anything, mock.AnythingOfType("[]string")).Return([]*regression.Regression{}, nil)

	req := &FileBugRequest{
		Component: "invalid",
		Keys:      []string{"1"},
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

	regStore.On("GetSubscriptionsForRegressions", mock.Anything, mock.AnythingOfType("[]string")).Return([]string{"1"}, []int64{1}, []*pb.Subscription{
		{
			BugComponent: "1325852",
		},
	}, nil)
	regStore.On("GetByIDs", mock.Anything, mock.AnythingOfType("[]string")).Return([]*regression.Regression{}, nil)

	req := &FileBugRequest{
		Title:       "Test Bug",
		Description: "This is a test bug.",
		Assignee:    "test@google.com",
		Ccs:         []string{"test2@google.com"},
		Keys:        []string{"1"},
	}

	_, err = s.FileBug(context.Background(), req)
	require.Error(t, err)
}

func TestFileBug_RequestBody(t *testing.T) {
	s, regStore, ts, receivedReq, receivedCommentReq := createIssueTrackerForTestInterceptRequests(t)
	defer ts.Close()

	regStore.On("GetSubscriptionsForRegressions", mock.Anything, mock.AnythingOfType("[]string")).Return([]string{"1"}, []int64{1}, []*pb.Subscription{
		{
			BugComponent: "8765",
			BugLabels:    []string{"BerfDevTest"},
			ContactEmail: "assignee@google.com",
		},
	}, nil)

	regStore.On("GetByIDs", mock.Anything, mock.AnythingOfType("[]string")).Return([]*regression.Regression{
		{
			Low:  nil,
			High: nil,
			Frame: &frame.FrameResponse{
				DataFrame: &dataframe.DataFrame{
					ParamSet: paramtools.NewReadOnlyParamSet(sampleParamsetMap),
				},
			},
			LowStatus:        regression.TriageStatus{},
			HighStatus:       regression.TriageStatus{},
			Id:               "1",
			CommitNumber:     12345,
			PrevCommitNumber: 12333,
			AlertId:          321,
			Bugs:             []types.RegressionBug{},
			AllBugsFetched:   false,
			CreationTime:     time.Time{},
			MedianBefore:     0,
			MedianAfter:      0,
			IsImprovement:    false,
			ClusterType:      "",
		},
	}, nil)

	req := &FileBugRequest{
		Title:       "Test Bug Title",
		Description: "Test Bug Description",
		Component:   "5678",
		Ccs:         []string{"cc1@google.com", "cc2@google.com"},
		Keys:        []string{"1"},
	}

	_, err := s.FileBug(context.Background(), req)
	require.NoError(t, err)

	require.Equal(t, "Test Bug Title", receivedReq.IssueState.Title)
	// Description is overriden.
	require.Contains(t, receivedReq.IssueComment.Comment, "Link to graph with regressions")
	require.Contains(t, receivedCommentReq.Comment, "Link to graph by bugID")
	require.Contains(t, receivedCommentReq.Comment, "12345")
	// TODO(b/454614028) Change it to regStore value once migration is done.
	defaultComponentId := int64(1325852)
	// Note that componentID is overriden by the default value
	require.Equal(t, defaultComponentId, receivedReq.IssueState.ComponentId)
	require.Equal(t, "assignee@google.com", receivedReq.IssueState.Assignee.EmailAddress)
	require.Len(t, receivedReq.IssueState.Ccs, 3)
	require.Equal(t, "cc1@google.com", receivedReq.IssueState.Ccs[0].EmailAddress)
	require.Equal(t, "cc2@google.com", receivedReq.IssueState.Ccs[1].EmailAddress)
	require.Equal(t, "assignee@google.com", receivedReq.IssueState.Ccs[2].EmailAddress)
}

func TestFileBug_EmptyDescription(t *testing.T) {
	s, regStore, ts, receivedReq, receivedCommentReq := createIssueTrackerForTestInterceptRequests(t)
	defer ts.Close()

	req := &FileBugRequest{
		Title:     "Test Bug Title",
		Component: "5678",
		Assignee:  "assignee@google.com",
		Ccs:       []string{"cc1@google.com", "cc2@google.com"},
		Keys:      []string{"key1", "key2"},
	}

	alertIDs := make([]int64, len(req.Keys))

	regStore.On("GetSubscriptionsForRegressions", mock.Anything, mock.AnythingOfType("[]string")).Return(req.Keys, alertIDs, []*pb.Subscription{
		{
			BugComponent: "8765",
		},
	}, nil)
	regStore.On("GetByIDs", mock.Anything, mock.AnythingOfType("[]string")).Return([]*regression.Regression{}, nil)

	_, err := s.FileBug(context.Background(), req)
	require.NoError(t, err)

	require.Contains(t, receivedReq.IssueComment.Comment, "http://test.com/u?anomalyIDs=key1,key2")
	require.Contains(t, receivedCommentReq.Comment, "12345")
}

func TestFileBug_EmptyDescriptionTooManyKeys(t *testing.T) {
	s, regStore, ts, receivedReq, receivedCommentReq := createIssueTrackerForTestInterceptRequests(t)
	defer ts.Close()

	keys := []string{}
	alertIDs := []int64{}
	for i := 0; i < 200; i++ {
		keys = append(keys, "aLongKeyThatWillMakeTheUrlExceedTheMaximumLength")
		alertIDs = append(alertIDs, int64(i))
	}
	req := &FileBugRequest{
		Title:     "Test Bug Title",
		Component: "5678",
		Assignee:  "assignee@google.com",
		Ccs:       []string{"cc1@google.com", "cc2@google.com"},
		Keys:      keys,
	}

	regStore.On("GetSubscriptionsForRegressions", mock.Anything, mock.AnythingOfType("[]string")).Return(req.Keys, alertIDs, []*pb.Subscription{
		{
			BugComponent: "8765",
		},
	}, nil)
	regStore.On("GetByIDs", mock.Anything, mock.AnythingOfType("[]string")).Return([]*regression.Regression{}, nil)

	_, err := s.FileBug(context.Background(), req)
	require.NoError(t, err)

	require.Contains(t, receivedReq.IssueComment.Comment, "The link to a graph with all regressions would be too long.")
	require.Contains(t, receivedCommentReq.Comment, "12345")
}

func TestFileBug_DeduplicateBots(t *testing.T) {
	s, regStore, ts, receivedReq, _ := createIssueTrackerForTestInterceptRequests(t)
	defer ts.Close()

	r := &regression.Regression{
		Low:  nil,
		High: nil,
		Frame: &frame.FrameResponse{
			DataFrame: &dataframe.DataFrame{
				ParamSet: paramtools.NewReadOnlyParamSet(sampleParamsetMap),
			},
		},
		LowStatus:        regression.TriageStatus{},
		HighStatus:       regression.TriageStatus{},
		Id:               "1",
		CommitNumber:     12345,
		PrevCommitNumber: 12333,
		AlertId:          321,
		Bugs:             []types.RegressionBug{},
		AllBugsFetched:   false,
		CreationTime:     time.Time{},
		MedianBefore:     0,
		MedianAfter:      0,
		IsImprovement:    false,
		ClusterType:      "",
	}
	regStore.On("GetByIDs", mock.Anything, mock.AnythingOfType("[]string")).Return([]*regression.Regression{
		r, r, // multiple regressions with the same bot
	}, nil)

	regStore.On("GetSubscriptionsForRegressions", mock.Anything, mock.AnythingOfType("[]string")).Return([]string{"1"}, []int64{1}, []*pb.Subscription{
		{
			BugComponent: "8765",
		},
	}, nil)

	req := &FileBugRequest{
		Title:       "Test Bug",
		Description: "This is a test bug.",
		Component:   "1234",
		Assignee:    "test@google.com",
		Ccs:         []string{"test2@google.com"},
		Keys:        []string{"1"},
	}

	_, err := s.FileBug(context.Background(), req)
	require.NoError(t, err)
	require.Contains(t, receivedReq.IssueComment.Comment, "Bots for regressions of this bug")
	// Assert bots are deduplicated
	require.True(t, strings.Count(receivedReq.IssueComment.Comment, sampleParamsetMap["bot"]) == 1)
}

func TestFileBug_SelectSubscription(t *testing.T) {
	s, regStore, ts, receivedReq, _ := createIssueTrackerForTestInterceptRequests(t)
	defer ts.Close()

	regStore.On("GetByIDs", mock.Anything, mock.AnythingOfType("[]string")).Return([]*regression.Regression{}, nil)

	regStore.On("GetSubscriptionsForRegressions", mock.Anything, mock.AnythingOfType("[]string")).Return([]string{"1"}, []int64{1}, []*pb.Subscription{
		{
			BugComponent: "111",
			BugPriority:  2,
			BugSeverity:  2,
			ContactEmail: "def@google.com",
			BugLabels:    []string{"BerfDevTest"},
		},
		{
			BugComponent: "222",
			BugPriority:  1,
			BugSeverity:  2,
			ContactEmail: "abc@google.com",
			BugLabels:    []string{"BerfDevTest"},
		},
	}, nil)

	req := &FileBugRequest{
		Keys: []string{"1"},
	}

	_, err := s.FileBug(context.Background(), req)
	require.NoError(t, err)

	// Note that componentID is overriden by the default value
	defaultComponentId := int64(1325852)
	require.Equal(t, defaultComponentId, receivedReq.IssueState.ComponentId)
	require.Equal(t, "P1", receivedReq.IssueState.Priority)
	require.Equal(t, "S2", receivedReq.IssueState.Severity)
	require.Equal(t, "abc@google.com", receivedReq.IssueState.Assignee.EmailAddress)
	require.Len(t, receivedReq.IssueState.Ccs, 2)
	require.Equal(t, "def@google.com", receivedReq.IssueState.Ccs[0].EmailAddress)
	require.Equal(t, "abc@google.com", receivedReq.IssueState.Ccs[1].EmailAddress)
}

func TestFileBug_SelectSubscription_SamePrio(t *testing.T) {
	s, regStore, ts, receivedReq, _ := createIssueTrackerForTestInterceptRequests(t)
	defer ts.Close()

	regStore.On("GetByIDs", mock.Anything, mock.AnythingOfType("[]string")).Return([]*regression.Regression{}, nil)

	regStore.On("GetSubscriptionsForRegressions", mock.Anything, mock.AnythingOfType("[]string")).Return([]string{"1"}, []int64{1}, []*pb.Subscription{
		{
			BugComponent: "111",
			BugPriority:  2,
			BugSeverity:  2,
			ContactEmail: "def@google.com",
			BugLabels:    []string{"BerfDevTest"},
		},
		{
			BugComponent: "222",
			BugPriority:  2,
			BugSeverity:  1,
			ContactEmail: "abc@google.com",
			BugLabels:    []string{"BerfDevTest"},
		},
	}, nil)

	req := &FileBugRequest{
		Keys: []string{"1"},
	}

	_, err := s.FileBug(context.Background(), req)
	require.NoError(t, err)

	// Note that componentID is overriden by the default value
	defaultComponentId := int64(1325852)
	require.Equal(t, defaultComponentId, receivedReq.IssueState.ComponentId)
	require.Equal(t, "P2", receivedReq.IssueState.Priority)
	require.Equal(t, "S1", receivedReq.IssueState.Severity)
	require.Equal(t, "abc@google.com", receivedReq.IssueState.Assignee.EmailAddress)
	require.Len(t, receivedReq.IssueState.Ccs, 2)
	require.Equal(t, "def@google.com", receivedReq.IssueState.Ccs[0].EmailAddress)
	require.Equal(t, "abc@google.com", receivedReq.IssueState.Ccs[1].EmailAddress)
}

func TestFileBug_SelectSubscription_BerfDevTest(t *testing.T) {
	s, regStore, ts, receivedReq, _ := createIssueTrackerForTestInterceptRequests(t)
	defer ts.Close()

	regStore.On("GetByIDs", mock.Anything, mock.AnythingOfType("[]string")).Return([]*regression.Regression{}, nil)

	regStore.On("GetSubscriptionsForRegressions", mock.Anything, mock.AnythingOfType("[]string")).Return([]string{"1"}, []int64{1}, []*pb.Subscription{
		{
			BugComponent: "111",
			BugPriority:  2,
			BugSeverity:  2,
			ContactEmail: "def@google.com",
			BugLabels:    []string{"BerfDevTest"},
		},
		{
			BugComponent: "222",
			BugPriority:  1,
			BugSeverity:  2,
			ContactEmail: "abc@google.com",
			BugLabels:    []string{"BerfDevTest"},
		},
	}, nil)

	req := &FileBugRequest{
		Keys: []string{"1"},
	}

	_, err := s.FileBug(context.Background(), req)
	require.NoError(t, err)

	// Note that componentID is overriden by the default value
	defaultComponentId := int64(1325852)
	require.Equal(t, defaultComponentId, receivedReq.IssueState.ComponentId)
	require.Equal(t, "P1", receivedReq.IssueState.Priority)
	require.Equal(t, "S2", receivedReq.IssueState.Severity)
	require.Equal(t, "abc@google.com", receivedReq.IssueState.Assignee.EmailAddress)
	require.Len(t, receivedReq.IssueState.Ccs, 2)
	require.Equal(t, "def@google.com", receivedReq.IssueState.Ccs[0].EmailAddress)
	require.Equal(t, "abc@google.com", receivedReq.IssueState.Ccs[1].EmailAddress)
}

// Remove this test after testRun check (bug label = BerfTest) is removed.
func TestFileBug_SelectSubscription_NotBerfDevTest(t *testing.T) {
	s, regStore, ts, receivedReq, _ := createIssueTrackerForTestInterceptRequests(t)
	defer ts.Close()

	regStore.On("GetByIDs", mock.Anything, mock.AnythingOfType("[]string")).Return([]*regression.Regression{}, nil)

	regStore.On("GetSubscriptionsForRegressions", mock.Anything, mock.AnythingOfType("[]string")).Return([]string{"1"}, []int64{1}, []*pb.Subscription{
		{
			BugComponent: "111",
			BugPriority:  1,
			BugSeverity:  1,
			ContactEmail: "def@google.com",
			BugLabels:    []string{"NotBerfDevTest"},
		},
		{
			BugComponent: "222",
			BugPriority:  1,
			BugSeverity:  2,
			ContactEmail: "abc@google.com",
			BugLabels:    []string{"BerfDevTest"},
		},
	}, nil)

	req := &FileBugRequest{
		Keys: []string{"1"},
	}

	_, err := s.FileBug(context.Background(), req)
	require.NoError(t, err)

	// Note that componentID is overriden by the default value
	defaultComponentId := int64(1325852)
	require.Equal(t, defaultComponentId, receivedReq.IssueState.ComponentId)
	require.Equal(t, "P1", receivedReq.IssueState.Priority)
	require.Equal(t, "S1", receivedReq.IssueState.Severity)
	// Values below are empty due to some subscription not being the test one.
	require.Equal(t, "", receivedReq.IssueState.Assignee.EmailAddress)
	require.Len(t, receivedReq.IssueState.Ccs, 0)
}

func TestFileBug_EmptySubscriptionsList(t *testing.T) {
	s, regStore, ts := createIssueTrackerForTest(t)
	defer ts.Close()

	regStore.On("GetSubscriptionsForRegressions", mock.Anything, mock.AnythingOfType("[]string")).Return([]string{"1"}, []int64{1}, []*pb.Subscription{}, nil)
	regStore.On("GetByIDs", mock.Anything, mock.AnythingOfType("[]string")).Return([]*regression.Regression{{}}, nil)

	req := &FileBugRequest{
		Keys: []string{"1"},
	}

	_, err := s.FileBug(context.Background(), req)
	require.ErrorContains(t, err, "did not find any subscriptions linked to those regressions")
}
