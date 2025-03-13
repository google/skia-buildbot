package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/pinpoint"
	pinpoint_pb "go.skia.org/infra/pinpoint/proto/v1"
)

const pinpointUrlTemplate = "https://pinpoint-dot-chromeperf.appspot.com/job/%s"

// pinpointApi provides a struct for handling Pinpoint requests.
type pinpointApi struct {
	loginProvider  alogin.Login
	legacyPinpoint *pinpoint.Client
	pinpointClient pinpoint_pb.PinpointClient
}

// NewPinpointApi returns a new instance of the pinpointApi struct.
func NewPinpointApi(loginProvider alogin.Login, legacyPinpoint *pinpoint.Client, pinpointClient pinpoint_pb.PinpointClient) pinpointApi {
	return pinpointApi{
		loginProvider:  loginProvider,
		legacyPinpoint: legacyPinpoint,
		pinpointClient: pinpointClient,
	}
}

// RegisterHandlers registers the api handlers for their respective routes.
func (api pinpointApi) RegisterHandlers(router *chi.Mux) {
	router.Post("/_/bisect/create", api.createBisectHandler)
	router.HandleFunc("/p/", api.pinpointBisectionHandler)
}

// createBisectHandler takes the POST'd create bisect request
// then it calls Pinpoint Service API to create bisect job and returns the job id and job url.
func (api pinpointApi) createBisectHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	defer r.Body.Close()

	if !api.loginProvider.HasRole(r, roles.Bisecter) {
		http.Error(w, "User is not logged in or is not authorized to start bisect.", http.StatusForbidden)
		return
	}

	// Most of the fields from the Chromeperf bisect request are compatible with
	// the Skia Pinpoint request.
	var cbr pinpoint_pb.ScheduleBisectRequest
	if err := json.NewDecoder(r.Body).Decode(&cbr); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON", http.StatusInternalServerError)
		return
	}

	// AlertIDs aren't supported in Skia Pinpoint request.
	// Statistic is updated to AggregationMethod, see updateFieldsForCatapult()
	// at //pinpoint/go/service/validation.go.
	switch {
	case cbr.Statistic == "avg":
		cbr.AggregationMethod = "mean"
	case cbr.Statistic != "":
		cbr.AggregationMethod = cbr.Statistic
	}

	exec, err := api.pinpointClient.ScheduleBisection(ctx, &cbr)
	if err != nil {
		httputils.ReportError(w, err, "Error scheduling bisection.", http.StatusInternalServerError)
		return
	}

	// Because we use the same job id as part of the Pinpoint URL, we can format
	// it and return it to be backwards compatible, but it'll be a 404 until
	// the job is actually complete and the writeback succeeds.
	resp := map[string]string{
		"jobId":  exec.GetJobId(),
		"jobUrl": fmt.Sprintf(pinpointUrlTemplate, exec.GetJobId()),
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to write or encode output: %s", err)
	}
}

// pinpointBisectionHandler handles a pinpoint bisection request and passes it on to the backend service.
func (api pinpointApi) pinpointBisectionHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	ctx, span := trace.StartSpan(ctx, "schedulePinpointBisectionRequest")
	defer span.End()

	// TODO(ashwinpv) Get the request data from incoming request.
	resp, err := api.pinpointClient.QueryBisection(ctx, &pinpoint_pb.QueryBisectRequest{})
	if err != nil {
		httputils.ReportError(w, err, "Error scheduling bisection.", 500)
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to write or encode output: %s", err)
	}
}
