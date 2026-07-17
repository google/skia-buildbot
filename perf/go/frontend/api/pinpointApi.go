package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/kube/go/authproxy"
	"go.skia.org/infra/kube/go/authproxy/protoheader"
	backendClient "go.skia.org/infra/perf/go/backend/client"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/pinpoint/go/pinpoint"
	pinpoint_pb "go.skia.org/infra/pinpoint/proto/v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

// pinpointApi provides a struct for handling Pinpoint requests.
type pinpointApi struct {
	loginProvider  alogin.Login
	pinpointClient *pinpoint.Client
	newClient      pinpoint_pb.PinpointClient
	devMode        bool
}

// NewPinpointApi returns a new instance of the pinpointApi struct.
func NewPinpointApi(loginProvider alogin.Login, pinpointClient *pinpoint.Client, devMode bool) *pinpointApi {
	newClient, err := backendClient.NewPinpointClient("", devMode || config.Config.Demo)
	if err != nil {
		sklog.Errorf("Failed to create new pinpoint client: %s", err)
	}
	return &pinpointApi{
		loginProvider:  loginProvider,
		pinpointClient: pinpointClient,
		newClient:      newClient,
		devMode:        devMode,
	}
}

func (api *pinpointApi) getNewPinpointClient() (pinpoint_pb.PinpointClient, error) {
	if api.newClient == nil {
		var err error
		api.newClient, err = backendClient.NewPinpointClient("", api.devMode || config.Config.Demo)
		if err != nil {
			return nil, skerr.Wrapf(err, "failed to create new pinpoint client")
		}
	}
	return api.newClient, nil
}

// RegisterHandlers registers the api handlers for their respective routes.
func (api *pinpointApi) RegisterHandlers(router *chi.Mux) {
	router.Post("/_/bisect/create", api.createBisectHandler)
	router.Post("/_/try", api.createTryJobHandler)
	router.HandleFunc("/p", api.pinpointBisectionHandler)
}

// checkAuthorized verifies if the user is authorized to perform the action.
func (api *pinpointApi) checkAuthorized(w http.ResponseWriter, r *http.Request, action string) bool {
	// TODO(b/404880786): Create a PinpointUser role and deprecate Bisecter.
	if !config.Config.Demo && !api.devMode && !api.loginProvider.HasRole(r, roles.Bisecter) {
		http.Error(w, fmt.Sprintf("User is not logged in or is not authorized to %s.", action), http.StatusForbidden)
		return false
	}
	return true
}

func getContextWithAuthHeaders(r *http.Request, timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	md := metadata.MD{}

	var user string
	if vals := getHeaderValuesCaseInsensitive(r, authproxy.WebAuthHeaderName); len(vals) > 0 {
		user = vals[0]
	} else if vals := getHeaderValuesCaseInsensitive(r, authproxy.GoogAuthenticatedUserEmailHeaderName); len(vals) > 0 {
		user = strings.TrimPrefix(vals[0], "accounts.google.com:")
	}

	if user != "" {
		md.Set(authproxy.WebAuthHeaderName, user)

		h := &protoheader.Header{Email: user}
		b, err := proto.Marshal(h)
		if err != nil {
			sklog.Errorf("Failed to marshal identity header proto for %s: %v", user, err)
		} else {
			encoded := base64.RawURLEncoding.EncodeToString(b)
			val := encoded + ".sig"
			md.Set(authproxy.EndpointAPIUserInfoHeaderName, val)
			md.Set("X-UberProxy-Signed-UpTick", val)
		}

		if vals := getHeaderValuesCaseInsensitive(r, authproxy.GoogAuthenticatedUserEmailHeaderName); len(vals) > 0 {
			md.Set(authproxy.GoogAuthenticatedUserEmailHeaderName, vals[0])
		} else {
			md.Set(authproxy.GoogAuthenticatedUserEmailHeaderName, "accounts.google.com:"+user)
		}

		if roles := getHeaderValuesCaseInsensitive(r, authproxy.WebAuthRoleHeaderName); len(roles) > 0 {
			md.Set(authproxy.WebAuthRoleHeaderName, roles...)
		}
	}

	return metadata.NewOutgoingContext(ctx, md), cancel
}

func getHeaderValuesCaseInsensitive(r *http.Request, name string) []string {
	if vals := r.Header.Values(name); len(vals) > 0 && vals[0] != "" {
		return vals
	}
	lowerName := strings.ToLower(name)
	for k, v := range r.Header {
		if strings.ToLower(k) == lowerName && len(v) > 0 {
			return v
		}
	}
	return nil
}

// createTryJobHandler takes the POST'd to create a Pinpoint try job request
// then it calls legacy Pinpoint Service to create the job and returns the job id and job url.
func (api *pinpointApi) createTryJobHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := getContextWithAuthHeaders(r, defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	if !api.checkAuthorized(w, r, "start try job") {
		return
	}

	if api.pinpointClient == nil {
		err := skerr.Fmt("Pinpoint client has not been initialized.")
		httputils.ReportError(w, err, "Create try job is not enabled for this instance, please check configuration file.", http.StatusInternalServerError)
		return
	}

	var cbr pinpoint.TryJobCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&cbr); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusBadRequest)
		return
	}
	sklog.Debugf("Got request of creating bisect job: %+v", cbr)

	isNewPinpoint := cbr.UseNewPinpoint
	if isNewPinpoint {
		repoUrl := getRepositoryUrl(cbr.Repository)
		startCommit := &pinpoint_pb.CombinedCommit{
			Main: &pinpoint_pb.Commit{
				GitHash:    cbr.BaseGitHash,
				Repository: repoUrl,
			},
		}
		endCommit := &pinpoint_pb.CombinedCommit{
			Main: &pinpoint_pb.Commit{
				GitHash:    cbr.EndGitHash,
				Repository: repoUrl,
			},
		}
		project := cbr.Project
		if project == "" {
			project = "chromium"
		}
		newReq := &pinpoint_pb.SchedulePairwiseRequest{
			StartCommit:         startCommit,
			EndCommit:           endCommit,
			Configuration:       cbr.Configuration,
			Benchmark:           cbr.Benchmark,
			Story:               cbr.Story,
			Project:             project,
			BugId:               cbr.BugId,
			UserEmail:           cbr.User,
			JobName:             cbr.Name,
			BaseExtraArgs:       cbr.ExtraTestArgs,
			ExperimentExtraArgs: cbr.ExtraTestArgs,
		}

		newClient, err := api.getNewPinpointClient()
		if err != nil || newClient == nil {
			httputils.ReportError(w, err, "Failed to connect to new pinpoint backend.", http.StatusInternalServerError)
			return
		}
		newResp, err := newClient.SchedulePairwise(ctx, newReq)
		if err != nil {
			httputils.ReportError(w, err, err.Error(), http.StatusInternalServerError)
			return
		}
		jobId := newResp.JobId

		response := &pinpoint.CreatePinpointResponse{
			JobID:  jobId,
			JobURL: getNewPinpointJobURL(jobId),
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			sklog.Errorf("Failed to encode response for try job: %s", err)
		}
		return
	}

	resp, err := api.pinpointClient.CreateTryJob(ctx, &cbr)
	if err != nil {
		httputils.ReportError(w, err, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to encode response for try job: %s", err)
	}
}

// createBisectHandler takes the POST'd create bisect request
// then it calls Pinpoint Service API to create bisect job and returns the job id and job url.
func (api *pinpointApi) createBisectHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := getContextWithAuthHeaders(r, defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	if !api.checkAuthorized(w, r, "start bisect") {
		return
	}

	if api.pinpointClient == nil {
		err := skerr.Fmt("Pinpoint client has not been initialized.")
		httputils.ReportError(w, err, "Create bisect is not enabled for this instance, please check configuration file.", http.StatusInternalServerError)
		return
	}

	var cbr pinpoint.BisectJobCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&cbr); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusBadRequest)
		return
	}
	sklog.Debugf("Got request of creating bisect job: %+v", cbr)

	isNewPinpoint := cbr.UseNewPinpoint
	if isNewPinpoint {
		project := cbr.Project
		if project == "" {
			project = "chromium"
		}
		newReq := &pinpoint_pb.ScheduleBisectRequest{
			ComparisonMode:      cbr.ComparisonMode,
			StartGitHash:        cbr.StartGitHash,
			EndGitHash:          cbr.EndGitHash,
			Configuration:       cbr.Configuration,
			Benchmark:           cbr.Benchmark,
			Story:               cbr.Story,
			Chart:               cbr.Chart,
			Statistic:           cbr.Statistic,
			ComparisonMagnitude: cbr.ComparisonMagnitude,
			Pin:                 cbr.Pin,
			Project:             project,
			BugId:               cbr.BugId,
			User:                cbr.User,
			ExtraArgs:           cbr.ExtraTestArgs,
		}

		newClient, err := api.getNewPinpointClient()
		if err != nil || newClient == nil {
			httputils.ReportError(w, err, "Failed to connect to new pinpoint backend.", http.StatusInternalServerError)
			return
		}
		newResp, err := newClient.ScheduleBisection(ctx, newReq)
		if err != nil {
			httputils.ReportError(w, err, err.Error(), http.StatusInternalServerError)
			return
		}
		jobId := newResp.JobId

		response := &pinpoint.CreatePinpointResponse{
			JobID:  jobId,
			JobURL: getNewPinpointJobURL(jobId),
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			sklog.Errorf("Failed to encode response for bisect job: %s", err)
		}
		return
	}

	isNewAnomaly := !preferLegacy(r)
	resp, err := api.pinpointClient.CreateBisect(ctx, &cbr, isNewAnomaly)
	if err != nil {
		httputils.ReportError(w, err, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to encode response for bisect job: %s", err)
	}
}

// pinpointBisectionHandler handles a pinpoint bisection request and passes it on to the backend service.
func (api *pinpointApi) pinpointBisectionHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := getContextWithAuthHeaders(r, defaultDatabaseTimeout)
	defer cancel()
	ctx, span := trace.StartSpan(ctx, "schedulePinpointBisectionRequest")
	defer span.End()

	if !api.checkAuthorized(w, r, "query bisection") {
		return
	}

	newPinpointClient, err := api.getNewPinpointClient()
	if err != nil {
		httputils.ReportError(w, err, "Error scheduling bisection.", http.StatusInternalServerError)
		return
	}

	// TODO(ashwinpv) Get the request data from incoming request.
	resp, err := newPinpointClient.QueryBisection(ctx, &pinpoint_pb.QueryBisectRequest{})
	if err != nil {
		httputils.ReportError(w, err, "Error scheduling bisection.", http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to encode response for query bisection: %s", err)
	}
}

func getRepositoryUrl(repo string) string {
	if config.Config.GitRepoConfig.URL != "" && (repo == "" || repo == config.Config.GitRepoConfig.Dir) {
		return config.Config.GitRepoConfig.URL
	}
	switch repo {
	case "chromium", "":
		return "https://chromium.googlesource.com/chromium/src.git"
	case "v8":
		return "https://chromium.googlesource.com/v8/v8.git"
	case "webrtc":
		return "https://webrtc.googlesource.com/src.git"
	case "skia":
		return "https://skia.googlesource.com/skia.git"
	default:
		return repo
	}
}

func getNewPinpointJobURL(jobId string) string {
	uiHost := config.Config.TemporalConfig.UiHostUrl
	if uiHost == "" {
		return ""
	}
	namespace := config.Config.TemporalConfig.Namespace
	if namespace == "" {
		namespace = "perf-internal"
	}
	return fmt.Sprintf("%s/namespaces/%s/workflows/%s", uiHost, namespace, jobId)
}
