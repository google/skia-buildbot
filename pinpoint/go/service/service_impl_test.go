package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pb "go.skia.org/infra/pinpoint/proto/v1"
)

func TestJSONHandler_UnknownEndpoints_ShouldReturn404(t *testing.T) {
	ctx := context.Background()
	svc := New()

	jh, err := NewJSONHandler(ctx, svc)
	require.Nil(t, err)

	rr := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/pinpoint/v1/fake-endpoint", nil)
	require.Nil(t, err)

	jh.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// TODO(b/322047067): Fix this and improve test coverage.
func TestJSONHandler_KnownEndpoints_ShouldForwardRequests(t *testing.T) {
	ctx := context.Background()
	svc := New()

	jh, err := NewJSONHandler(ctx, svc)
	require.Nil(t, err)

	expect := func(method, endpoint, body string, want int) {
		rr := httptest.NewRecorder()
		req, err := http.NewRequest(method, endpoint, strings.NewReader(body))
		assert.Nil(t, err)
		jh.ServeHTTP(rr, req)
		assert.Equal(t, want, rr.Code)
	}

	// TODO(b/322047067): Return code should be 200 with actual response body.
	expect("POST", "/pinpoint/v1/schedule", "{}", http.StatusNotImplemented)
	expect("GET", "/pinpoint/v1/query", "", http.StatusNotImplemented)
	expect("GET", "/pinpoint/v1/legacy-job", "", http.StatusNotImplemented)
}

func TestScheduleBisection_InvalidRequests_ShouldError(t *testing.T) {
	ctx := context.Background()
	svc := New()

	resp, err := svc.ScheduleBisection(ctx, &pb.ScheduleBisectRequest{})
	assert.Nil(t, resp)
	// TODO(b/322047067): empty requests should return a response with reasons.
	assert.ErrorContains(t, err, "not implemented")

	// TODO(b/322047067): Add requests with invalid fields
}

func TestQueryBisection_ExistingJob_ShouldReturnDetails(t *testing.T) {
	ctx := context.Background()
	svc := New()

	expect := func(req *pb.QueryBisectRequest, want *pb.BisectExecution, desc string) {
		resp, err := svc.QueryBisection(ctx, req)
		// TODO(b/322047067): remove this once implemented, err should be nil
		assert.ErrorContains(t, err, "not implemented")

		// TODO(b/322047067): the response should match the expected
		assert.Nil(t, resp, desc)
	}

	// TODO(b/322047067): Add more combinations of query request and fix expected responses.
	expect(&pb.QueryBisectRequest{
		JobId: "TBD ID",
	}, nil, "should return job status")

}

func TestQueryBisection_NonExistingJob_ShouldError(t *testing.T) {
	ctx := context.Background()
	svc := New()

	resp, err := svc.QueryBisection(ctx, &pb.QueryBisectRequest{
		JobId: "non-exist ID",
	})
	// TODO(b/322047067): change this to correct error message
	assert.ErrorContains(t, err, "not implemented", "Error should indicate job doesn't exist.")
	assert.Nil(t, resp, "Non-existed Job ID shouldn't contain any response.")

	resp, err = svc.QueryBisection(ctx, &pb.QueryBisectRequest{})
	// TODO(b/322047067): change this to correct error message
	assert.ErrorContains(t, err, "not implemented", "Empty Job ID should error.")
	assert.Nil(t, resp, "Empty Job ID shouldn't contain any response.")
}
