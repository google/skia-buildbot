package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/psrefresh"
	"go.skia.org/infra/perf/go/ui/frame"
)

// queryApi provides a struct handle api requests related to the query dialog.
type queryApi struct {
	paramsetRefresher psrefresh.ParamSetRefresher
}

// NewQueryApi returns a new instance of queryApi struct.
func NewQueryApi(paramsetRefresher psrefresh.ParamSetRefresher) queryApi {
	return queryApi{
		paramsetRefresher: paramsetRefresher,
	}
}

// RegisterHandlers registers the api handlers for their respective routes.
func (api queryApi) RegisterHandlers(router *chi.Mux) {
	router.HandleFunc("/_/initpage", api.initpageHandler)
	router.Post("/_/count", api.countHandler)
	router.Post("/_/nextParamList", api.nextParamListHandler)
}

// NextParamListHandlerRequest is the JSON format for NextParamListHandler request.
type NextParamListHandlerRequest struct {
	Query string `json:"q"`
}

// NextParamListHandlerResponse is the JSON format for NextParamListHandler response.
type NextParamListHandlerResponse struct {
	Count    int                         `json:"count"`
	Paramset paramtools.ReadOnlyParamSet `json:"paramset"`
}

// CountHandlerRequest is the JSON format for the countHandler request.
type CountHandlerRequest struct {
	Q     string `json:"q"`
	Begin int    `json:"begin"`
	End   int    `json:"end"`
}

// CountHandlerResponse is the JSON format if the countHandler response.
type CountHandlerResponse struct {
	Count    int                         `json:"count"`
	Paramset paramtools.ReadOnlyParamSet `json:"paramset"`
}

// PreflightQuery generates the query and calls PreflightQuery on dfBuilder
func (api *queryApi) PreflightQuery(ctx context.Context, w http.ResponseWriter, qs string) (int, paramtools.ReadOnlyParamSet, error) {
	u, err := url.ParseQuery(qs)
	if err != nil {
		httputils.ReportError(w, err, "Invalid URL query.", http.StatusInternalServerError)
		return 0, nil, err
	}
	q, err := query.New(u)
	if err != nil {
		httputils.ReportError(w, err, "Invalid query.", http.StatusInternalServerError)
		return 0, nil, err
	}

	fullPS := api.getParamSet()
	if qs == "" {
		return 0, fullPS, nil
	} else {
		count, ps, err := api.paramsetRefresher.GetParamSetForQuery(ctx, q, u)
		if err != nil {
			httputils.ReportError(w, err, fmt.Sprintf("Failed to Preflight the query: %s", err), http.StatusBadRequest)
			return 0, nil, err
		}
		return int(count), filterParamSetIfNeeded(ps.Freeze()), nil
	}
}

// nextParamListHandler takes the POST'd query and runs that against the current
// dataframe and returns how many traces match the query.
// Notice that nextParamListHandler is a chromeperf specific version of countHander.
// The differences here are:
//   - in the UI, the parameter fields take the user intpus in strict order.
//     e.g., end users are expect to first input benchmark, and then bot,
//     and then measurement, etc. The order is defined in include_params in the config file.
//   - the reponse does not includes all paramsets. It only returns the values for the
//     'next' parameter in order.
func (api *queryApi) nextParamListHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	var npr NextParamListHandlerRequest
	if err := json.NewDecoder(r.Body).Decode(&npr); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}

	nextParam, err := findNextParamInQueryString(npr.Query)
	if err != nil {
		httputils.ReportError(w, err, "Error in findNextParamInQueryString.", http.StatusInternalServerError)
		return
	}

	count, ps, err := api.PreflightQuery(ctx, w, npr.Query)
	if err != nil {
		httputils.ReportError(w, err, "Error in nextParamListHandler.", http.StatusInternalServerError)
		return
	}
	resp := NextParamListHandlerResponse{
		Count: count,
	}
	if nextParam == "" {
		// There is no next parameter. No filtering is needed.
		resp.Paramset = map[string][]string{}
	} else {
		// There's a next parameter, but there's no matching paramset.
		if _, ok := ps[nextParam]; !ok {
			resp.Paramset = map[string][]string{}
		} else {
			resp.Paramset = map[string][]string{nextParam: ps[nextParam]}
		}

	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		httputils.ReportError(w, err, "Failed to encode nextparam response.", http.StatusInternalServerError)
	}
}

// countHandler takes the POST'd query and runs that against the current
// dataframe and returns how many traces match the query.
func (api *queryApi) countHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
	defer cancel()
	w.Header().Set("Content-Type", "application/json")

	var cr CountHandlerRequest
	if err := json.NewDecoder(r.Body).Decode(&cr); err != nil {
		httputils.ReportError(w, err, "Failed to decode JSON.", http.StatusInternalServerError)
		return
	}

	count, ps, err := api.PreflightQuery(ctx, w, cr.Q)
	if err != nil {
		sklog.Errorf("Error in nextParamListHandler: %s", err)
	}
	resp := CountHandlerResponse{
		Count:    count,
		Paramset: ps,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to encode paramset: %s", err)
	}
}

// initpageHandler returns the paramset to initialize the page.
func (f *queryApi) initpageHandler(w http.ResponseWriter, _ *http.Request) {
	resp := &frame.FrameResponse{
		DataFrame: &dataframe.DataFrame{
			ParamSet: f.getParamSet(),
		},
		Skps: []int{},
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to encode paramset: %s", err)
	}
}

// given a query string from UI, find the next paramter which is:
//   - not in the query string, and
//   - in the include_params next in order.
//
// e.g., if end user has selected values for 'benchmark' and 'bot', and the
// include_params is ["benchmark","bot","test","subtest_1","subtest_2","subtest_3"],
// the next expected parameter is 'test'.
func findNextParamInQueryString(qs string) (string, error) {
	// If no include_params is defined in config, we have no way to tell
	// which is 'next'.
	if config.Config.QueryConfig.IncludedParams == nil {
		err := errors.New("no included parameter list in config")
		sklog.Error("No included parameter list in config.", err)
		return "", err
	}
	qKeyValues, err := url.ParseQuery(qs)
	if err != nil {
		sklog.Error("Invalid URL query. %s", err)
		return "", err
	}
	for _, key := range config.Config.QueryConfig.IncludedParams {
		// when scanning the parameter list in order, if the included parameter
		// key is not in the query, it is the next we are looking for.
		if _, ok := qKeyValues[key]; !ok {
			return key, nil
		}
	}
	// all included parameter keys are in the query.
	return "", nil
}

// getParamSet returns a fresh paramtools.ParamSet that represents all the
// traces stored in the two most recent tiles in the trace store. It is filtered
// if such filtering is turned on in the config.
func (api *queryApi) getParamSet() paramtools.ReadOnlyParamSet {
	paramSet := api.paramsetRefresher.GetAll()

	return filterParamSetIfNeeded(paramSet)
}

// filterParamSetIfNeeded filters the paramset if any filters have been specified in
// the query config.
func filterParamSetIfNeeded(paramSet paramtools.ReadOnlyParamSet) paramtools.ReadOnlyParamSet {
	if config.Config.QueryConfig.IncludedParams != nil {
		filteredParamSet := paramtools.NewParamSet()
		for _, key := range config.Config.QueryConfig.IncludedParams {
			if val, ok := paramSet[key]; ok {
				existing, exists := filteredParamSet[key]
				if exists {
					existing = append(existing, val...)
				} else {
					existing = val
				}
				filteredParamSet[key] = existing
			}
		}

		paramSet = paramtools.ReadOnlyParamSet(filteredParamSet)
	}

	return paramSet
}
