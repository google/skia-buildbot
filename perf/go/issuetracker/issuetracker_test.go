package issuetracker

import (
	"context"
	"encoding/json"
	"fmt"
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
	v1 "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/regression"
	regMocks "go.skia.org/infra/perf/go/regression/mocks"

	regrShortcutMocks "go.skia.org/infra/perf/go/regrshortcut/mocks"
	pb "go.skia.org/infra/perf/go/subscription/proto/v1"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
	userissueMocks "go.skia.org/infra/perf/go/userissue/mocks"
)

var sampleParamsetMap = map[string]string{
	"bot":                   "botxyz",
	"benchmark":             "benchmark",
	"story":                 "story",
	"measurement":           "measurement",
	"stat":                  "stat",
	"test":                  "test",
	"subtest_1":             "subtest_1",
	"improvement_direction": "up",
}

func getMockRegressions(n int) []*regression.Regression {
	res := make([]*regression.Regression, n)
	for i := 0; i < n; i++ {
		res[i] = &regression.Regression{
			Frame: &frame.FrameResponse{
				DataFrame: &dataframe.DataFrame{
					ParamSet: paramtools.NewReadOnlyParamSet(sampleParamsetMap),
				},
			},
		}
	}
	return res
}

var testSubOwner = "sergeirudenkov@google.com"
var testSubEmail = "berf-issuetracker-testing@google.com"

func createIssueTrackerForTest(t *testing.T) (*issueTrackerImpl, *regMocks.Store, *userissueMocks.Store, *httptest.Server) {
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
	regrShortcutStore := &regrShortcutMocks.Store{}
	userIssueStore := &userissueMocks.Store{}
	return &issueTrackerImpl{
		client:                client,
		FetchAnomaliesFromSql: true,
		regStore:              regStore,
		regrShortcutStore:     regrShortcutStore,
		userIssueStore:        userIssueStore,
	}, regStore, userIssueStore, ts
}

func createIssueTrackerForTestInterceptRequests(t *testing.T) (*issueTrackerImpl, *regMocks.Store, *regrShortcutMocks.Store, *userissueMocks.Store, *httptest.Server, *issuetracker.Issue, *issuetracker.IssueComment) {
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
	regrShortcutStore := &regrShortcutMocks.Store{}
	userIssueStore := &userissueMocks.Store{}
	s := &issueTrackerImpl{
		client:                c,
		FetchAnomaliesFromSql: true,
		regStore:              regStore,
		regrShortcutStore:     regrShortcutStore,
		userIssueStore:        userIssueStore,
		urlBase:               "http://test.com",
	}
	return s, regStore, regrShortcutStore, userIssueStore, ts, &receivedReq, &receivedCommentReq
}

func TestFileBug_Success(t *testing.T) {
	s, regStore, _, ts := createIssueTrackerForTest(t)
	defer ts.Close()

	regStore.On("GetSubscriptionsForRegressions", mock.Anything, mock.AnythingOfType("[]string")).Return([]string{"1"}, []*pb.Subscription{
		{
			BugComponent: "1235",
		},
	}, nil)
	regStore.On("GetByIDs", mock.Anything, mock.AnythingOfType("[]string")).Return(getMockRegressions(1), nil)

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
	s, _, _, ts := createIssueTrackerForTest(t)
	defer ts.Close()
	_, err := s.FileBug(context.Background(), nil)
	require.Error(t, err)
}

func TestFileBug_InvalidComponent(t *testing.T) {
	s, regStore, _, ts := createIssueTrackerForTest(t)
	defer ts.Close()

	regStore.On("GetSubscriptionsForRegressions", mock.Anything, mock.AnythingOfType("[]string")).Return([]string{"1"}, []*pb.Subscription{
		{
			BugComponent: "invalid",
		},
	}, nil)
	regStore.On("GetByIDs", mock.Anything, mock.AnythingOfType("[]string")).Return(getMockRegressions(1), nil)

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

	regStore.On("GetSubscriptionsForRegressions", mock.Anything, mock.AnythingOfType("[]string")).Return([]string{"1"}, []*pb.Subscription{
		{
			BugComponent: "1325852",
		},
	}, nil)
	regStore.On("GetByIDs", mock.Anything, mock.AnythingOfType("[]string")).Return(getMockRegressions(1), nil)

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
	s, regStore, _, _, ts, receivedReq, receivedCommentReq := createIssueTrackerForTestInterceptRequests(t)
	defer ts.Close()

	regStore.On("GetSubscriptionsForRegressions", mock.Anything, mock.AnythingOfType("[]string")).Return([]string{"1"}, []*pb.Subscription{
		{
			BugComponent: "8765",
			BugLabels:    []string{"BerfDevTest"},
			ContactEmail: testSubOwner,
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
	require.Contains(t, receivedReq.IssueComment.Comment, "test.com")
	require.Contains(t, receivedCommentReq.Comment, "Link to graph by bugID")
	require.Contains(t, receivedCommentReq.Comment, "12345")
	// Note that componentID is overriden by the default value
	require.Equal(t, int64(8765), receivedReq.IssueState.ComponentId)
	require.Equal(t, testSubEmail, receivedReq.IssueState.Assignee.EmailAddress)
	require.Len(t, receivedReq.IssueState.Ccs, 3)
	require.Equal(t, "cc1@google.com", receivedReq.IssueState.Ccs[0].EmailAddress)
	require.Equal(t, "cc2@google.com", receivedReq.IssueState.Ccs[1].EmailAddress)
	require.Equal(t, testSubOwner, receivedReq.IssueState.Ccs[2].EmailAddress)
}

func TestFileBug_EmptyDescription(t *testing.T) {
	s, regStore, regrShortcutStore, _, ts, receivedReq, receivedCommentReq := createIssueTrackerForTestInterceptRequests(t)
	defer ts.Close()

	req := &FileBugRequest{
		Title:     "Test Bug Title",
		Component: "5678",
		Assignee:  testSubOwner,
		Ccs:       []string{"cc1@google.com", "cc2@google.com"},
		Keys:      []string{"key1", "key2"},
	}

	regStore.On("GetSubscriptionsForRegressions", mock.Anything, mock.AnythingOfType("[]string")).Return(req.Keys, []*pb.Subscription{
		{
			BugComponent: "8765",
		},
	}, nil)
	regStore.On("GetByIDs", mock.Anything, mock.AnythingOfType("[]string")).Return(getMockRegressions(2), nil)
	regrShortcutStore.On("Create", mock.Anything, mock.AnythingOfType("[]string")).Return("deadbeef", nil)

	_, err := s.FileBug(context.Background(), req)
	require.NoError(t, err)

	require.Contains(t, receivedReq.IssueComment.Comment, "http://test.com/u?sid=deadbeef")
	require.Contains(t, receivedCommentReq.Comment, "12345")
}

func TestFileBug_EmptyDescriptionTooManyKeys(t *testing.T) {
	s, regStore, regrShortcutStore, _, ts, receivedReq, receivedCommentReq := createIssueTrackerForTestInterceptRequests(t)
	defer ts.Close()

	keys := []string{}
	for i := 0; i < 200; i++ {
		keys = append(keys, "aLongKeyThatWillMakeTheUrlExceedTheMaximumLength")
	}
	req := &FileBugRequest{
		Title:     "Test Bug Title",
		Component: "5678",
		Assignee:  testSubOwner,
		Ccs:       []string{"cc1@google.com", "cc2@google.com"},
		Keys:      keys,
	}

	regStore.On("GetSubscriptionsForRegressions", mock.Anything, mock.AnythingOfType("[]string")).Return(req.Keys, []*pb.Subscription{
		{
			BugComponent: "8765",
		},
	}, nil)
	regStore.On("GetByIDs", mock.Anything, mock.AnythingOfType("[]string")).Return(getMockRegressions(200), nil)
	regrShortcutStore.On("Create", mock.Anything, mock.AnythingOfType("[]string")).Return("deadbeef", nil)

	_, err := s.FileBug(context.Background(), req)
	require.NoError(t, err)

	require.Contains(t, receivedReq.IssueComment.Comment, "http://test.com/u?sid=deadbeef")
	require.Contains(t, receivedCommentReq.Comment, "12345")
}

func TestFileBug_SelectSubscription(t *testing.T) {
	s, regStore, _, _, ts, receivedReq, _ := createIssueTrackerForTestInterceptRequests(t)
	defer ts.Close()

	regStore.On("GetByIDs", mock.Anything, mock.AnythingOfType("[]string")).Return(getMockRegressions(1), nil)

	regStore.On("GetSubscriptionsForRegressions", mock.Anything, mock.AnythingOfType("[]string")).Return([]string{"1"}, []*pb.Subscription{
		{
			BugComponent: "111",
			BugPriority:  2,
			BugSeverity:  2,
			ContactEmail: testSubOwner,
			BugLabels:    []string{"BerfDevTest"},
		},
		{
			BugComponent: "222",
			BugPriority:  1,
			BugSeverity:  2,
			ContactEmail: testSubOwner,
			BugLabels:    []string{"BerfDevTest"},
		},
	}, nil)

	req := &FileBugRequest{
		Keys: []string{"1"},
	}

	_, err := s.FileBug(context.Background(), req)
	require.NoError(t, err)

	// Note that componentID is overriden by the default value
	require.Equal(t, int64(222), receivedReq.IssueState.ComponentId)
	require.Equal(t, "P1", receivedReq.IssueState.Priority)
	require.Equal(t, "S2", receivedReq.IssueState.Severity)
	require.Equal(t, testSubEmail, receivedReq.IssueState.Assignee.EmailAddress)
	require.Len(t, receivedReq.IssueState.Ccs, 2)
	require.Equal(t, testSubOwner, receivedReq.IssueState.Ccs[0].EmailAddress)
	require.Equal(t, testSubOwner, receivedReq.IssueState.Ccs[1].EmailAddress)
}

func TestFileBug_SelectSubscription_SamePrio(t *testing.T) {
	s, regStore, _, _, ts, receivedReq, _ := createIssueTrackerForTestInterceptRequests(t)
	defer ts.Close()

	regStore.On("GetByIDs", mock.Anything, mock.AnythingOfType("[]string")).Return(getMockRegressions(1), nil)

	regStore.On("GetSubscriptionsForRegressions", mock.Anything, mock.AnythingOfType("[]string")).Return([]string{"1"}, []*pb.Subscription{
		{
			BugComponent: "111",
			BugPriority:  2,
			BugSeverity:  2,
			ContactEmail: testSubOwner,
			BugLabels:    []string{"BerfDevTest"},
		},
		{
			BugComponent: "222",
			BugPriority:  2,
			BugSeverity:  1,
			ContactEmail: testSubOwner,
			BugLabels:    []string{"BerfDevTest"},
		},
	}, nil)

	req := &FileBugRequest{
		Keys: []string{"1"},
	}

	_, err := s.FileBug(context.Background(), req)
	require.NoError(t, err)

	require.Equal(t, int64(222), receivedReq.IssueState.ComponentId)
	require.Equal(t, "P2", receivedReq.IssueState.Priority)
	require.Equal(t, "S1", receivedReq.IssueState.Severity)
	require.Equal(t, testSubEmail, receivedReq.IssueState.Assignee.EmailAddress)
	require.Len(t, receivedReq.IssueState.Ccs, 2)
	require.Equal(t, testSubOwner, receivedReq.IssueState.Ccs[0].EmailAddress)
	require.Equal(t, testSubOwner, receivedReq.IssueState.Ccs[1].EmailAddress)
}

// Remove this test after testRun check (bug label = BerfTest) is removed.
func TestFileBug_SelectSubscription_NotBerfDevTest(t *testing.T) {
	s, regStore, _, _, ts, receivedReq, _ := createIssueTrackerForTestInterceptRequests(t)
	defer ts.Close()
	s.OverrideComponent = true

	regStore.On("GetByIDs", mock.Anything, mock.AnythingOfType("[]string")).Return(getMockRegressions(1), nil)

	regStore.On("GetSubscriptionsForRegressions", mock.Anything, mock.AnythingOfType("[]string")).Return([]string{"1"}, []*pb.Subscription{
		{
			BugComponent: "111",
			BugPriority:  1,
			BugSeverity:  1,
			ContactEmail: "otheremail@google.com",
			BugLabels:    []string{"NotBerfDevTest"},
		},
		{
			BugComponent: "222",
			BugPriority:  1,
			BugSeverity:  2,
			ContactEmail: testSubOwner,
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
	require.Nil(t, receivedReq.IssueState.Assignee)
	require.Len(t, receivedReq.IssueState.Ccs, 0)
}

func TestFileBug_EmptySubscriptionsList(t *testing.T) {
	s, regStore, _, ts := createIssueTrackerForTest(t)
	defer ts.Close()

	regStore.On("GetSubscriptionsForRegressions", mock.Anything, mock.AnythingOfType("[]string")).Return([]string{"1"}, []*pb.Subscription{}, nil)
	regStore.On("GetByIDs", mock.Anything, mock.AnythingOfType("[]string")).Return(getMockRegressions(1), nil)

	req := &FileBugRequest{
		Keys: []string{"1"},
	}

	_, err := s.FileBug(context.Background(), req)
	require.ErrorContains(t, err, "did not find any subscriptions linked to those regressions")
}

func TestFileUserIssue_Success(t *testing.T) {
	s, _, _, userIssueStore, ts, receivedReq, receivedCommentReq := createIssueTrackerForTestInterceptRequests(t)
	defer ts.Close()

	req := &CreateUserIssueRequest{
		TraceKey:       "test-trace",
		CommitPosition: 100,
		Assignee:       "user@google.com",
	}

	issueID, err := s.FileUserIssue(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, 12345, issueID)

	require.Equal(t, "Trace ID test-trace shows a potential regression at commit position 100.", receivedReq.IssueState.Title)
	require.Equal(t, "user@google.com", receivedReq.IssueState.Assignee.EmailAddress)
	require.Contains(t, receivedCommentReq.Comment, "Link to trace by bugID")
	userIssueStore.AssertExpectations(t)
}

func TestFileUserIssue_NilRequest(t *testing.T) {
	s, _, _, ts := createIssueTrackerForTest(t)
	defer ts.Close()
	_, err := s.FileUserIssue(context.Background(), nil)
	require.Error(t, err)
}

func TestGenerateAnomTableHeaders(t *testing.T) {
	headers := generateAnomTableHeaders()

	lines := strings.Split(headers, "\n")
	require.GreaterOrEqual(t, len(lines), 3)

	headerLine := lines[1]
	dividerLine := lines[2]

	headerPipes := strings.Count(headerLine, "|")
	dividerPipes := strings.Count(dividerLine, "|")
	dividerDashes := strings.Count(dividerLine, "---")

	require.Equal(t, headerPipes, dividerPipes)
	// There are 7 columns, so 8 pipes and 7 dividers
	require.Equal(t, 7, dividerDashes)
}

func TestDescribeAnomaly(t *testing.T) {
	anomaly := &v1.Anomaly{
		Paramset: map[string]string{
			"bot":         "bot1",
			"benchmark":   "bench1",
			"measurement": "meas1",
			"story":       "story1",
		},
		MedianBefore: 10.0,
		MedianAfter:  15.0,
		StartCommit:  100,
		EndCommit:    101,
	}
	s := &issueTrackerImpl{}
	desc := s.describeAnomaly(context.TODO(), anomaly)

	pipes := strings.Count(desc, "|")
	// Expected 7 columns separated by 8 pipes (6 inner pipes + 2 optional outer)
	require.Equal(t, 8, pipes)
}

func TestIntersectionFooter_NonEmptyIntersection(t *testing.T) {
	s := &issueTrackerImpl{
		commitHashRangeFormatter: func(ctx context.Context, begin, end int64) string {
			return fmt.Sprintf("http://commits/%d/%d", begin, end)
		},
	}

	regData := []*regression.Regression{
		{PrevCommitNumber: types.CommitNumber(10), CommitNumber: types.CommitNumber(20)},
		{PrevCommitNumber: types.CommitNumber(15), CommitNumber: types.CommitNumber(25)},
		{PrevCommitNumber: types.CommitNumber(12), CommitNumber: types.CommitNumber(22)},
	}

	footer := s.intersectionFooter(context.TODO(), regData)
	require.Contains(t, footer, "Common commit range of all regressions in this bug")
	require.Contains(t, footer, "http://commits/15/20")
}

func TestIntersectionFooter_EmptyIntersection(t *testing.T) {
	s := &issueTrackerImpl{
		commitHashRangeFormatter: func(ctx context.Context, begin, end int64) string {
			return fmt.Sprintf("http://commits/%d/%d", begin, end)
		},
	}

	regData := []*regression.Regression{
		{PrevCommitNumber: types.CommitNumber(10), CommitNumber: types.CommitNumber(20)},
		{PrevCommitNumber: types.CommitNumber(25), CommitNumber: types.CommitNumber(30)},
	}

	footer := s.intersectionFooter(context.TODO(), regData)
	require.Contains(t, footer, "Commit intersection of regressions in this bug is empty!")
}

func TestNewIssueTracker_FileBug_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := &issuetracker.Issue{
			IssueId: 54321,
		}
		err := json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	}))
	defer ts.Close()

	ctx := context.Background()
	regStore := &regMocks.Store{}
	regrShortcutStore := &regrShortcutMocks.Store{}
	userIssueStore := &userissueMocks.Store{}

	// Mocking regression store
	regStore.On("GetSubscriptionsForRegressions", mock.Anything, mock.AnythingOfType("[]string")).Return([]string{"1"}, []*pb.Subscription{
		{
			BugComponent: "1235",
		},
	}, nil)
	regStore.On("GetByIDs", mock.Anything, mock.AnythingOfType("[]string")).Return(getMockRegressions(1), nil)

	cfg := config.IssueTrackerConfig{}
	tracker, err := NewIssueTracker(ctx, IssueTrackerDeps{
		Cfg:                      cfg,
		FetchAnomaliesFromSql:    true,
		OverrideBugComponent:     false,
		RegStore:                 regStore,
		RegrShortcutStore:        regrShortcutStore,
		UserIssueStore:           userIssueStore,
		DevMode:                  true,
		UrlBase:                  "http://test.com",
		CommitHashRangeFormatter: nil,
	})
	require.NoError(t, err)

	tracker.(*issueTrackerImpl).client.BasePath = ts.URL

	req := &FileBugRequest{
		Title:       "Test Bug via NewIssueTracker",
		Description: "This is a test bug.",
		Component:   "1234",
		Assignee:    "test@google.com",
		Ccs:         []string{"test2@google.com"},
		Keys:        []string{"1"},
	}

	issueID, err := tracker.FileBug(ctx, req)
	require.NoError(t, err)
	require.Equal(t, 54321, issueID)
}

func TestCreateIssue_Success(t *testing.T) {
	var receivedReq issuetracker.Issue
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &receivedReq)
		require.NoError(t, err)

		w.Header().Set("Content-Type", "application/json")
		resp := &issuetracker.Issue{
			IssueId: 999,
		}
		err = json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	}))
	defer ts.Close()

	c, err := issuetracker.NewService(context.Background(), option.WithEndpoint(ts.URL), option.WithoutAuthentication())
	require.NoError(t, err)

	s := &issueTrackerImpl{
		client: c,
	}

	req := &CreateIssueRequest{
		Title:       "Test Title",
		Description: "Test Description",
		ComponentId: 123,
		Priority:    "P2",
		Severity:    "S2",
		Reporter:    "reporter@google.com",
		Ccs:         []string{"cc@google.com"},
		AccessLevel: "LIMIT_VIEW_TRUSTED",
		Status:      "NEW",
	}

	issueId, err := s.CreateIssue(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, int64(999), issueId)

	require.Equal(t, "Test Title", receivedReq.IssueState.Title)
	require.Equal(t, "Test Description", receivedReq.IssueComment.Comment)
	require.Equal(t, int64(123), receivedReq.IssueState.ComponentId)
	require.Equal(t, "P2", receivedReq.IssueState.Priority)
	require.Equal(t, "S2", receivedReq.IssueState.Severity)
	require.Equal(t, "reporter@google.com", receivedReq.IssueState.Reporter.EmailAddress)
	require.Len(t, receivedReq.IssueState.Ccs, 1)
	require.Equal(t, "cc@google.com", receivedReq.IssueState.Ccs[0].EmailAddress)
	require.Equal(t, "LIMIT_VIEW_TRUSTED", receivedReq.IssueState.AccessLimit.AccessLevel)
	require.Equal(t, "NEW", receivedReq.IssueState.Status)
}

func TestModifyIssue_BothStatusAndComment(t *testing.T) {
	var receivedReq issuetracker.ModifyIssueRequest
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &receivedReq)
		require.NoError(t, err)

		w.Header().Set("Content-Type", "application/json")
		resp := &issuetracker.Issue{
			IssueId: 123,
		}
		err = json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	}))
	defer ts.Close()

	c, err := issuetracker.NewService(context.Background(), option.WithEndpoint(ts.URL), option.WithoutAuthentication())
	require.NoError(t, err)

	s := &issueTrackerImpl{
		client: c,
	}

	req := &ModifyIssueRequest{
		IssueId: 123,
		Status:  "OBSOLETE",
		Comment: "Closing this issue",
	}

	err = s.ModifyIssue(context.Background(), req)
	require.NoError(t, err)

	require.Equal(t, "OBSOLETE", receivedReq.Add.Status)
	require.Equal(t, "status", receivedReq.AddMask)
	require.Equal(t, "Closing this issue", receivedReq.IssueComment.Comment)
}

func TestModifyIssue_OnlyComment(t *testing.T) {
	var receivedReq issuetracker.ModifyIssueRequest
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &receivedReq)
		require.NoError(t, err)

		w.Header().Set("Content-Type", "application/json")
		resp := &issuetracker.IssueComment{
			CommentNumber: 1,
		}
		err = json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	}))
	defer ts.Close()

	c, err := issuetracker.NewService(context.Background(), option.WithEndpoint(ts.URL), option.WithoutAuthentication())
	require.NoError(t, err)

	s := &issueTrackerImpl{
		client: c,
	}

	req := &ModifyIssueRequest{
		IssueId: 123,
		Comment: "Just a comment",
	}

	err = s.ModifyIssue(context.Background(), req)
	require.NoError(t, err)

	require.Equal(t, "Just a comment", receivedReq.IssueComment.Comment)
}
