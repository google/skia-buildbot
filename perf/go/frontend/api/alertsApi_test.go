package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	subscriptionMocks "go.skia.org/infra/perf/go/subscription/mocks"
	subscriptionProtoV1 "go.skia.org/infra/perf/go/subscription/proto/v1"
)

func TestFrontendUniqSubscriptionHandler_Success(t *testing.T) {
	subMock := subscriptionMocks.NewStore(t)
	subMock.On("GetAllSubscriptions", testutils.AnyContext).Return(
		[]*subscriptionProtoV1.Subscription{
			{
				Name:         "Test Subscription 1",
				Revision:     "abcd",
				BugLabels:    []string{"A", "B"},
				Hotlists:     []string{"C", "D"},
				BugComponent: "Component1>Subcomponent1",
				BugPriority:  1,
				BugSeverity:  2,
				BugCcEmails: []string{
					"abcd@efg.com",
					"1234@567.com",
				},
				ContactEmail: "test@owner.com",
			},
			{
				Name:         "Test Subscription 2",
				Revision:     "bcde",
				BugLabels:    []string{"A", "B"},
				Hotlists:     []string{"C", "D"},
				BugComponent: "Component1>Subcomponent1",
				BugPriority:  1,
				BugSeverity:  2,
				BugCcEmails: []string{
					"abcd@efg.com",
					"1234@567.com",
				},
				ContactEmail: "test@owner.com",
			},
		}, nil)
	a := NewAlertsApi(nil, nil, nil, nil, subMock, nil)
	w := httptest.NewRecorder()

	r := httptest.NewRequest("GET", "/_/allsubscriptions", nil)
	a.subscriptionsHandler(w, r)

	require.Equal(t, http.StatusOK, w.Result().StatusCode)
	require.Contains(t, w.Body.String(), "Test Subscription 1")
	require.Contains(t, w.Body.String(), "Test Subscription 2")
}
