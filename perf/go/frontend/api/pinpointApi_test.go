// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	aloginMocks "go.skia.org/infra/go/alogin/mocks"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/kube/go/authproxy"
	"go.skia.org/infra/kube/go/authproxy/protoheader"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/pinpoint/go/pinpoint"
	pinpoint_pb "go.skia.org/infra/pinpoint/proto/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

type fakeNewPinpointClient struct {
	pinpoint_pb.PinpointClient
	scheduleBisectReq   *pinpoint_pb.ScheduleBisectRequest
	scheduleBisectRes   *pinpoint_pb.BisectExecution
	schedulePairwiseReq *pinpoint_pb.SchedulePairwiseRequest
	schedulePairwiseRes *pinpoint_pb.PairwiseExecution
}

func (f *fakeNewPinpointClient) ScheduleBisection(ctx context.Context, in *pinpoint_pb.ScheduleBisectRequest, opts ...grpc.CallOption) (*pinpoint_pb.BisectExecution, error) {
	f.scheduleBisectReq = in
	return f.scheduleBisectRes, nil
}

func (f *fakeNewPinpointClient) SchedulePairwise(ctx context.Context, in *pinpoint_pb.SchedulePairwiseRequest, opts ...grpc.CallOption) (*pinpoint_pb.PairwiseExecution, error) {
	f.schedulePairwiseReq = in
	return f.schedulePairwiseRes, nil
}

func TestCreateTryJob_NotLoggedIn(t *testing.T) {
	require := require.New(t)

	login := &aloginMocks.Login{}
	// HasRole returns false to mock unauthorized user
	login.On("HasRole", mock.Anything, mock.Anything).Return(false)

	// Instantiate pinpointApi
	api := NewPinpointApi(login, &pinpoint.Client{}, false)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/_/try/", nil)

	api.createTryJobHandler(w, r)

	require.Equal(http.StatusForbidden, w.Code)
	require.Contains(w.Body.String(), "User is not logged in or is not authorized")
	login.AssertExpectations(t)
}

func TestGetContextWithAuthHeaders(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/_/bisect/", nil)
	r.Header.Set(authproxy.WebAuthHeaderName, "user@google.com")
	r.Header.Set(authproxy.WebAuthRoleHeaderName, "bisecter")
	r.Header.Set(authproxy.GoogAuthenticatedUserEmailHeaderName, "accounts.google.com:user@google.com")

	ctx, cancel := getContextWithAuthHeaders(r, defaultDatabaseTimeout)
	defer cancel()

	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(t, ok)
	require.Equal(t, []string{"user@google.com"}, md.Get(authproxy.WebAuthHeaderName))
	require.Equal(t, []string{"bisecter"}, md.Get(authproxy.WebAuthRoleHeaderName))
	require.Len(t, md.Get(authproxy.EndpointAPIUserInfoHeaderName), 1)
	require.Equal(t, []string{"accounts.google.com:user@google.com"}, md.Get(authproxy.GoogAuthenticatedUserEmailHeaderName))
}

func TestGetContextWithAuthHeaders_SynthesizesMissingHeadersFromGoogUser(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/_/bisect/", nil)
	r.Header.Set(authproxy.GoogAuthenticatedUserEmailHeaderName, "accounts.google.com:user@google.com")
	r.Header.Set(authproxy.WebAuthRoleHeaderName, "bisecter")

	ctx, cancel := getContextWithAuthHeaders(r, defaultDatabaseTimeout)
	defer cancel()

	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(t, ok)
	require.Equal(t, []string{"user@google.com"}, md.Get(authproxy.WebAuthHeaderName))
	require.Equal(t, []string{"bisecter"}, md.Get(authproxy.WebAuthRoleHeaderName))
	require.Equal(t, []string{"accounts.google.com:user@google.com"}, md.Get(authproxy.GoogAuthenticatedUserEmailHeaderName))

	epUserList := md.Get(authproxy.EndpointAPIUserInfoHeaderName)
	require.Len(t, epUserList, 1)
	epUser := epUserList[0]

	// Simulate ProtoHeader.LoggedInAs decoding logic on perf-be
	reqForBe := httptest.NewRequest(http.MethodPost, "/pinpoint.v1.Pinpoint/SchedulePairwise", nil)
	reqForBe.Header.Set("X-Endpoint-API-UserInfo", epUser)

	headerValue := reqForBe.Header.Get("X-Endpoint-API-UserInfo")
	parts := strings.Split(headerValue, ".")
	require.Len(t, parts, 2)

	b, err := base64.RawURLEncoding.DecodeString(parts[0])
	require.NoError(t, err)

	var h protoheader.Header
	err = proto.Unmarshal(b, &h)
	require.NoError(t, err)
	require.Equal(t, "user@google.com", h.Email)
}

func TestGetContextWithAuthHeaders_SynthesizesMissingHeadersFromWebAuthUser(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/_/bisect/", nil)
	r.Header.Set(authproxy.WebAuthHeaderName, "user@google.com")

	ctx, cancel := getContextWithAuthHeaders(r, defaultDatabaseTimeout)
	defer cancel()

	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(t, ok)
	require.Equal(t, []string{"user@google.com"}, md.Get(authproxy.WebAuthHeaderName))
	require.Equal(t, []string{"accounts.google.com:user@google.com"}, md.Get(authproxy.GoogAuthenticatedUserEmailHeaderName))

	epUserList := md.Get(authproxy.EndpointAPIUserInfoHeaderName)
	require.Len(t, epUserList, 1)
	epUser := epUserList[0]

	reqForBe := httptest.NewRequest(http.MethodPost, "/pinpoint.v1.Pinpoint/SchedulePairwise", nil)
	reqForBe.Header.Set("X-Endpoint-API-UserInfo", epUser)

	headerValue := reqForBe.Header.Get("X-Endpoint-API-UserInfo")
	parts := strings.Split(headerValue, ".")
	require.Len(t, parts, 2)

	b, err := base64.RawURLEncoding.DecodeString(parts[0])
	require.NoError(t, err)

	var h protoheader.Header
	err = proto.Unmarshal(b, &h)
	require.NoError(t, err)
	require.Equal(t, "user@google.com", h.Email)
}

func TestCreateBisect_AutopushQueue_RoutesToNewPinpointInsteadOfLegacyCatapult(t *testing.T) {
	origConfig := config.Config
	t.Cleanup(func() {
		config.Config = origConfig
	})
	config.Config = &config.InstanceConfig{
		TemporalConfig: config.TemporalConfig{
			UiHostUrl:         "https://skia-temporal-ui.corp.goog",
			Namespace:         "perf-internal",
			PinpointTaskQueue: "perf.perf-chrome-autopush.bisect",
		},
	}

	login := &aloginMocks.Login{}
	login.On("HasRole", mock.Anything, roles.Bisecter).Return(true)

	fakeNew := &fakeNewPinpointClient{
		scheduleBisectRes: &pinpoint_pb.BisectExecution{
			JobId: "autopush-workflow-uuid",
		},
	}
	api := &pinpointApi{
		loginProvider:  login,
		pinpointClient: &pinpoint.Client{},
		newClient:      fakeNew,
	}

	reqPayload := pinpoint.BisectJobCreateRequest{
		ComparisonMode: "performance",
		StartGitHash:   "aaaa1111",
		EndGitHash:     "bbbb2222",
		Configuration:  "mac-m1_mini_2020-perf",
		Benchmark:      "speedometer3",
		Story:          "Speedometer3",
		Chart:          "Score",
		BugId:          "1325852",
		UseNewPinpoint: false,
	}
	body, err := json.Marshal(reqPayload)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/_/bisect/create", bytes.NewBuffer(body))

	require.NotPanics(t, func() {
		api.createBisectHandler(w, r)
	})

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, fakeNew.scheduleBisectReq)
	require.Equal(t, "speedometer3", fakeNew.scheduleBisectReq.Benchmark)
	require.Equal(t, "1325852", fakeNew.scheduleBisectReq.BugId)

	var resp pinpoint.CreatePinpointResponse
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	require.Equal(t, "autopush-workflow-uuid", resp.JobID)
	require.Equal(t, "https://skia-temporal-ui.corp.goog/namespaces/perf-internal/workflows/autopush-workflow-uuid", resp.JobURL)
}

func TestCreateTryJob_AutopushQueue_RoutesToNewPinpointInsteadOfLegacyCatapult(t *testing.T) {
	origConfig := config.Config
	t.Cleanup(func() {
		config.Config = origConfig
	})
	config.Config = &config.InstanceConfig{
		TemporalConfig: config.TemporalConfig{
			UiHostUrl:         "https://skia-temporal-ui.corp.goog",
			Namespace:         "perf-internal",
			PinpointTaskQueue: "perf.perf-chrome-autopush.bisect",
		},
	}

	login := &aloginMocks.Login{}
	login.On("HasRole", mock.Anything, roles.Bisecter).Return(true)

	fakeNew := &fakeNewPinpointClient{
		schedulePairwiseRes: &pinpoint_pb.PairwiseExecution{
			JobId: "autopush-pairwise-uuid",
		},
	}
	api := &pinpointApi{
		loginProvider:  login,
		pinpointClient: &pinpoint.Client{},
		newClient:      fakeNew,
	}

	reqPayload := pinpoint.TryJobCreateRequest{
		Name:           "Autopush Try Job",
		BaseGitHash:    "aaaa1111",
		EndGitHash:     "bbbb2222",
		Configuration:  "mac-m1_mini_2020-perf",
		Benchmark:      "speedometer3",
		Story:          "Speedometer3",
		BugId:          "1325852",
		UseNewPinpoint: false,
	}
	body, err := json.Marshal(reqPayload)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/_/try", bytes.NewBuffer(body))

	require.NotPanics(t, func() {
		api.createTryJobHandler(w, r)
	})

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, fakeNew.schedulePairwiseReq)
	require.Equal(t, "speedometer3", fakeNew.schedulePairwiseReq.Benchmark)
	require.Equal(t, "1325852", fakeNew.schedulePairwiseReq.BugId)

	var resp pinpoint.CreatePinpointResponse
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	require.Equal(t, "autopush-pairwise-uuid", resp.JobID)
	require.Equal(t, "https://skia-temporal-ui.corp.goog/namespaces/perf-internal/workflows/autopush-pairwise-uuid", resp.JobURL)
}
