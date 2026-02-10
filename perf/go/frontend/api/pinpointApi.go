package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	backendClient "go.skia.org/infra/perf/go/backend/client"
	"go.skia.org/infra/perf/go/pinpoint"
	pinpoint_pb "go.skia.org/infra/pinpoint/proto/v1"
)

// pinpointApi provides a struct for handling Pinpoint requests.
type pinpointApi struct {
	loginProvider  alogin.Login
	pinpointClient *pinpoint.Client
}

// NewPinpointApi returns a new instance of the pinpointApi struct.
func NewPinpointApi(loginProvider alogin.Login, pinpointClient *pinpoint.Client) pinpointApi {
	return pinpointApi{
		loginProvider:  loginProvider,
		pinpointClient: pinpointClient,
	}
}

// RegisterHandlers registers the api handlers for their respective routes.
func (api pinpointApi) RegisterHandlers(router *chi.Mux) {
	router.Post("/_/bisect/create", api.createBisectHandler)
	router.Post("/_/try", api.createTryJobHandler)
	router.HandleFunc("/p", api.pinpointBisectionHandler)
}

// createTryJobHandler takes the POST'd to create a Pinpoint try job request
// then it calls legacy Pinpoint Service to create the job and returns the job id and job url.
func (api pinpointApi) createTryJobHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	// TODO(b/404880786): Create a PinpointUser role and deprecate Bisecter.
	if !api.loginProvider.HasRole(r, roles.Bisecter) {
		http.Error(w, "User is not logged in or is not authorized to start try job.", http.StatusForbidden)
		return
	}

	if api.pinpointClient == nil {
		err := skerr.Fmt("Pinpoint client has not been initialized.")
		httputils.ReportError(w, err, "Create try job is not enabled for this instance, please check configuration file.", http.StatusInternalServerError)
		return
	}

	var cbr pinpoint.CreateLegacyTryRequest
	if err := json.NewDecoder(r.Body).Decode(&cbr); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}
	sklog.Debugf("Got request of creating bisect job: %+v", cbr)

	resp, err := api.pinpointClient.CreateTryJob(ctx, cbr)
	if err != nil {
		// TODO(b/483368236): Pass the correct HTTP error code.
		httputils.ReportError(w, err, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to parse the response of creating A/B job: %s", err)
	}
}

// createBisectHandler takes the POST'd create bisect request
// then it calls Pinpoint Service API to create bisect job and returns the job id and job url.
func (api pinpointApi) createBisectHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	if !api.loginProvider.HasRole(r, roles.Bisecter) {
		http.Error(w, "User is not logged in or is not authorized to start bisect.", http.StatusForbidden)
		return
	}

	if api.pinpointClient == nil {
		err := skerr.Fmt("Pinpoint client has not been initialized.")
		httputils.ReportError(w, err, "Create bisect is not enabled for this instance, please check configuration file.", http.StatusInternalServerError)
		return
	}

	var cbr pinpoint.CreateBisectRequest
	if err := json.NewDecoder(r.Body).Decode(&cbr); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}
	sklog.Debugf("Got request of creating bisect job: %+v", cbr)

	resp, err := api.pinpointClient.CreateBisect(ctx, cbr)
	if err != nil {
		// TODO(b/483368236): Pass the correct HTTP error code.
		httputils.ReportError(w, err, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to parse the response of creating bisect job: %s", err)
	}
}

// pinpointBisectionHandler handles a pinpoint bisection request and passes it on to the backend service.
func (api pinpointApi) pinpointBisectionHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	ctx, span := trace.StartSpan(ctx, "schedulePinpointBisectionRequest")
	defer span.End()

	pinpointClient, err := backendClient.NewPinpointClient("")
	if err != nil {
		httputils.ReportError(w, err, "Error scheduling bisection.", 500)
	}

	// TODO(ashwinpv) Get the request data from incoming request.
	resp, err := pinpointClient.QueryBisection(ctx, &pinpoint_pb.QueryBisectRequest{})
	if err != nil {
		httputils.ReportError(w, err, "Error scheduling bisection.", 500)
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to write or encode output: %s", err)
	}
}
