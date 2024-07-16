package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/auditlog"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/bug"
	"go.skia.org/infra/perf/go/dryrun"
	"go.skia.org/infra/perf/go/notify"
	"go.skia.org/infra/perf/go/subscription"
)

// alertsApi provides a struct to manage api endpoints for Alerts.
type alertsApi struct {
	loginProvider  alogin.Login
	configProvider alerts.ConfigProvider
	alertStore     alerts.Store
	notifier       notify.Notifier
	subStore       subscription.Store
	dryrunRequests *dryrun.Requests
}

// NewAlertsApi returns a new instance of the alertsApi struct.
func NewAlertsApi(loginProvider alogin.Login, configProvider alerts.ConfigProvider, alertStore alerts.Store, notifier notify.Notifier, subStore subscription.Store, dryRunRequests *dryrun.Requests) alertsApi {
	return alertsApi{
		loginProvider:  loginProvider,
		configProvider: configProvider,
		alertStore:     alertStore,
		notifier:       notifier,
		subStore:       subStore,
		dryrunRequests: dryRunRequests,
	}
}

// RegisterHandlers registers the api handlers for their respective routes.
func (a alertsApi) RegisterHandlers(router *chi.Mux) {
	router.Get("/_/alert/list/{show}", a.alertListHandler)
	router.Get("/_/alert/new", a.alertNewHandler)
	router.Post("/_/alert/update", a.alertUpdateHandler)
	router.Post("/_/alert/delete/{id:[0-9]+}", a.alertDeleteHandler)
	router.Post("/_/alert/bug/try", a.alertBugTryHandler)
	router.Post("/_/alert/notify/try", a.alertNotifyTryHandler)
	router.Get("/_/subscriptions", a.subscriptionsHandler)
	router.Post("/_/dryrun/start", a.dryrunRequests.StartHandler)
}

// alertListHandler returns a list of alert configs in the database.
func (a alertsApi) alertListHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	show := chi.URLParam(r, "show")
	resp, err := a.configProvider.GetAllAlertConfigs(ctx, show == "true")
	if err != nil {
		httputils.ReportError(w, err, "Failed to retrieve alert configs.", http.StatusInternalServerError)
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to write JSON response: %s", err)
	}
}

// alertNewHandler returns a new empty alert config.
func (a alertsApi) alertNewHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(alerts.NewConfig()); err != nil {
		sklog.Errorf("Failed to write JSON response: %s", err)
	}
}

// AlertUpdateResponse is the JSON response when an Alert is created or udpated.
type AlertUpdateResponse struct {
	IDAsString string
}

// alertUpdateHandler updates the alert config data.
func (a alertsApi) alertUpdateHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	defer refreshConfigProvider(ctx, a.configProvider)
	w.Header().Set("Content-Type", "application/json")

	cfg := &alerts.Alert{}
	if err := json.NewDecoder(r.Body).Decode(cfg); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}

	if !a.isEditor(w, r, "alert-update", cfg) {
		return
	}

	if err := cfg.Validate(); err != nil {
		httputils.ReportError(w, err, "Invalid Alert", http.StatusInternalServerError)
	}

	if err := a.alertStore.Save(ctx, &alerts.SaveRequest{Cfg: cfg}); err != nil {
		httputils.ReportError(w, err, "Failed to save alerts.Config.", http.StatusInternalServerError)
	}
	err := json.NewEncoder(w).Encode(AlertUpdateResponse{
		IDAsString: cfg.IDAsString,
	})
	if err != nil {
		sklog.Errorf("Failed to write JSON response: %s", err)
	}
}

// alertDeleteHandler deletes the specified alert config.
func (a alertsApi) alertDeleteHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	defer refreshConfigProvider(ctx, a.configProvider)
	w.Header().Set("Content-Type", "application/json")

	sid := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(sid, 10, 64)
	if err != nil {
		httputils.ReportError(w, err, "Failed to parse alert id.", http.StatusInternalServerError)
	}

	if !a.isEditor(w, r, "alert-delete", sid) {
		return
	}

	if err := a.alertStore.Delete(ctx, int(id)); err != nil {
		httputils.ReportError(w, err, "Failed to delete the alerts.Config.", http.StatusInternalServerError)
		return
	}
}

// refreshConfigProvider refreshes the alert config provider.
func refreshConfigProvider(ctx context.Context, configProvider alerts.ConfigProvider) {
	err := configProvider.Refresh(ctx)
	if err != nil {
		sklog.Errorf("Error refreshing alert configs: %s", err)
	}
}

// TryBugRequest is a request to try a bug template URI.
type TryBugRequest struct {
	BugURITemplate string `json:"bug_uri_template"`
}

// TryBugResponse is response to a TryBugRequest.
type TryBugResponse struct {
	URL string `json:"url"`
}

// alertBugTryHandler attempts to dry run the bug creation flow for the alert config.
func (a alertsApi) alertBugTryHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	req := &TryBugRequest{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}

	if !a.isEditor(w, r, "alert-bug-try", req) {
		return
	}

	resp := &TryBugResponse{
		URL: bug.ExampleExpand(req.BugURITemplate),
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to encode response: %s", err)
	}
}

// alertNotifyTryHandler attempts to send a test notification based on the alert config.
func (a alertsApi) alertNotifyTryHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	req := &alerts.Alert{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}

	if !a.isEditor(w, r, "alert-notify-try", req) {
		return
	}

	if err := a.notifier.ExampleSend(ctx, req); err != nil {
		httputils.ReportError(w, err, "Failed to send notification: Have you given the service account for this instance Issue Editor permissions on the component?", http.StatusInternalServerError)
	}
}

// subscriptionsHandler is an API endpoint handler that fetches all the subscriptions from the db
func (a alertsApi) subscriptionsHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultDatabaseTimeout)
	defer cancel()
	ctx, span := trace.StartSpan(ctx, "subscriptionQueryRequest")
	defer span.End()

	subscriptionList, err := a.subStore.GetAllSubscriptions(ctx)
	if err != nil {
		httputils.ReportError(w, err, "Unable to fetch subscription", http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(subscriptionList); err != nil {
		sklog.Errorf("Failed to write or encode output: %s", err)
	}
}

func (a alertsApi) isEditor(w http.ResponseWriter, r *http.Request, action string, body interface{}) bool {
	user := a.loginProvider.LoggedInAs(r)
	if !a.loginProvider.HasRole(r, roles.Editor) {
		httputils.ReportError(w, fmt.Errorf("Not logged in."), "You must be logged in to complete this action.", http.StatusUnauthorized)
		return false
	}
	auditlog.LogWithUser(r, user.String(), action, body)
	return true
}
