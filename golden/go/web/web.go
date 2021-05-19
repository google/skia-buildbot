package web

import (
	"bytes"
	"context"
	"crypto/md5"
	"image"
	"image/png"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/gorilla/mux"
	lru "github.com/hashicorp/golang-lru"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/search2"
	search2_fe "go.skia.org/infra/golden/go/search2/frontend"
	"go.skia.org/infra/golden/go/search2/query"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/web/frontend"
)

const (
	// pageSize is the default page size used for pagination.
	pageSize = 20

	// maxPageSize is the maximum page size used for pagination.
	maxPageSize = 100

	// These params limit how anonymous (not logged-in) users can hit various endpoints.
	// We have two buckets of requests - cheap and expensive. Expensive stuff hits a database
	// or similar, where as cheap stuff is cached. These limits are shared by *all* endpoints
	// in a given bucket. See skbug.com/9476 for more.
	maxAnonQPSExpensive   = rate.Limit(0.01)
	maxAnonBurstExpensive = 50
	maxAnonQPSCheap       = rate.Limit(5.0)
	maxAnonBurstCheap     = 50
	// Special settings for RPCs serving the gerrit plugin. See skbug.com/10768 for more.
	maxAnonQPSGerritPlugin   = rate.Limit(200.0)
	maxAnonBurstGerritPlugin = 1000

	changelistSummaryCacheSize = 10000

	// RPCCallCounterMetric is the metric that should be used when counting how many times a given
	// RPC route is called from clients.
	RPCCallCounterMetric = "gold_rpc_call_counter"
)

type validateFields int

const (
	// FullFrontEnd means all fields should be set
	FullFrontEnd validateFields = iota
	// BaselineSubset means just the fields needed for Baseline Server should be set.
	BaselineSubset
)

// HandlersConfig holds the environment needed by the various http handler functions.
type HandlersConfig struct {
	DB            *pgxpool.Pool
	GCSClient     storage.GCSClient
	ReviewSystems []clstore.ReviewSystem
	Search2API    search2.API
}

// Handlers represents all the handlers (e.g. JSON endpoints) of Gold.
// It should be created by clients using NewHandlers.
type Handlers struct {
	HandlersConfig

	anonymousExpensiveQuota *rate.Limiter
	anonymousCheapQuota     *rate.Limiter
	anonymousGerritQuota    *rate.Limiter

	clSummaryCache *lru.Cache

	// These can be set for unit tests to simplify the testing.
	testingAuthAs string
}

// NewHandlers returns a new instance of Handlers.
func NewHandlers(conf HandlersConfig, val validateFields) (*Handlers, error) {
	// These fields are required by all types.
	if len(conf.ReviewSystems) == 0 {
		return nil, skerr.Fmt("ReviewSystems cannot be empty")
	}
	if conf.GCSClient == nil {
		return nil, skerr.Fmt("GCSClient cannot be nil")
	}
	if conf.DB == nil {
		return nil, skerr.Fmt("DB cannot be nil")
	}
	if val == FullFrontEnd {
		if conf.Search2API == nil {
			return nil, skerr.Fmt("Search2API cannot be nil")
		}
	}

	clcache, err := lru.New(changelistSummaryCacheSize)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return &Handlers{
		HandlersConfig:          conf,
		anonymousExpensiveQuota: rate.NewLimiter(maxAnonQPSExpensive, maxAnonBurstExpensive),
		anonymousCheapQuota:     rate.NewLimiter(maxAnonQPSCheap, maxAnonBurstCheap),
		anonymousGerritQuota:    rate.NewLimiter(maxAnonQPSGerritPlugin, maxAnonBurstGerritPlugin),
		clSummaryCache:          clcache,
		testingAuthAs:           "", // Just to be explicit that we do *not* bypass Auth.
	}, nil
}

// limitForAnonUsers blocks using the configured rate.Limiter for expensive queries.
func (wh *Handlers) limitForAnonUsers(r *http.Request) error {
	if login.LoggedInAs(r) != "" {
		return nil
	}
	return wh.anonymousExpensiveQuota.Wait(r.Context())
}

// cheapLimitForAnonUsers blocks using the configured rate.Limiter for cheap queries.
func (wh *Handlers) cheapLimitForAnonUsers(r *http.Request) error {
	if login.LoggedInAs(r) != "" {
		return nil
	}
	return wh.anonymousCheapQuota.Wait(r.Context())
}

// cheapLimitForGerritPlugin blocks using the configured rate.Limiter for queries for the
// Gerrit Plugin.
func (wh *Handlers) cheapLimitForGerritPlugin(r *http.Request) error {
	if login.LoggedInAs(r) != "" {
		return nil
	}
	return wh.anonymousGerritQuota.Wait(r.Context())
}

// ByBlameHandler2 takes the response from the SQL backend's GetBlamesForUntriagedDigests and
// converts it into the same format that the legacy version (v1) produced.
func (wh *Handlers) ByBlameHandler2(w http.ResponseWriter, r *http.Request) {
	if err := wh.limitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}
	ctx, span := trace.StartSpan(r.Context(), "web_ByBlameHandler2", trace.WithSampler(trace.AlwaysSample()))
	defer span.End()

	// Extract the corpus from the query parameters.
	corpus := ""
	if v := r.FormValue("query"); v != "" {
		if qp, err := url.ParseQuery(v); err != nil {
			httputils.ReportError(w, err, "invalid input", http.StatusBadRequest)
			return
		} else if corpus = qp.Get(types.CorpusField); corpus == "" {
			// If no corpus specified report an error.
			http.Error(w, "did not receive value for corpus", http.StatusBadRequest)
			return
		}
	} else {
		// If no corpus specified report an error.
		http.Error(w, "did not receive value for search query", http.StatusBadRequest)
		return
	}
	summary, err := wh.Search2API.GetBlamesForUntriagedDigests(ctx, corpus)
	if err != nil {
		httputils.ReportError(w, err, "Could not compute blames", http.StatusInternalServerError)
		return
	}
	result := frontend.ByBlameResponse{}
	for _, sr := range summary.Ranges {
		entry := frontend.ByBlameEntry{
			GroupID:  sr.CommitRange,
			NDigests: sr.TotalUntriagedDigests,
			NTests:   len(sr.AffectedGroupings),
			Commits:  sr.Commits,
		}
		var groupings []frontend.TestRollup
		for _, gr := range sr.AffectedGroupings {
			groupings = append(groupings, frontend.TestRollup{
				Test:         types.TestName(gr.Grouping[types.PrimaryKeyField]),
				Num:          gr.UntriagedDigests,
				SampleDigest: gr.SampleDigest,
			})
		}
		entry.AffectedTests = groupings
		result.Data = append(result.Data, entry)
	}
	sendJSONResponse(w, result)
}

// ChangelistsHandler returns the list of code_review.Changelists that have
// uploaded results to Gold (via TryJobs).
func (wh *Handlers) ChangelistsHandler(w http.ResponseWriter, r *http.Request) {
	if err := wh.limitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	values := r.URL.Query()
	offset, size, err := httputils.PaginationParams(values, 0, pageSize, maxPageSize)
	if err != nil {
		httputils.ReportError(w, err, "Invalid pagination params.", http.StatusInternalServerError)
		return
	}

	_, activeOnly := values["active"]
	cls, pagination, err := wh.getIngestedChangelists(r.Context(), offset, size, activeOnly)

	if err != nil {
		httputils.ReportError(w, err, "Retrieving changelists results failed.", http.StatusInternalServerError)
		return
	}

	response := frontend.ChangelistsResponse{
		Changelists:        cls,
		ResponsePagination: *pagination,
	}

	sendJSONResponse(w, response)
}

// getIngestedChangelists performs the core of the logic for ChangelistsHandler,
// by fetching N Changelists given an offset.
func (wh *Handlers) getIngestedChangelists(ctx context.Context, offset, size int, activeOnly bool) ([]frontend.Changelist, *httputils.ResponsePagination, error) {
	so := clstore.SearchOptions{
		StartIdx: offset,
		Limit:    size,
	}
	if activeOnly {
		so.OpenCLsOnly = true
	}

	grandTotal := 0
	var retCls []frontend.Changelist
	for _, system := range wh.ReviewSystems {
		cls, total, err := system.Store.GetChangelists(ctx, so)
		if err != nil {
			return nil, nil, skerr.Wrapf(err, "fetching Changelists from [%d:%d)", offset, offset+size)
		}

		for _, cl := range cls {
			retCls = append(retCls, frontend.ConvertChangelist(cl, system.ID, system.URLTemplate))
		}
		if grandTotal == clstore.CountMany || total == clstore.CountMany {
			grandTotal = clstore.CountMany
		} else {
			grandTotal += total
		}
	}

	pagination := &httputils.ResponsePagination{
		Offset: offset,
		Size:   size,
		Total:  grandTotal,
	}
	return retCls, pagination, nil
}

// PatchsetsAndTryjobsForCL2 returns a summary of the data we have collected
// for a given Changelist, specifically any TryJobs that have uploaded data
// to Gold belonging to various patchsets in it.
func (wh *Handlers) PatchsetsAndTryjobsForCL2(w http.ResponseWriter, r *http.Request) {
	if err := wh.limitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}
	// TODO(kjlubick)
}

// A list of CI systems we support. So far, the mapping of task ID to link is project agnostic. If
// that stops being the case, then we'll need to supply this mapping on a per-instance basis.
var cisTemplates = map[string]string{
	"cirrus":      "https://cirrus-ci.com/task/%s",
	"buildbucket": "https://cr-buildbucket.appspot.com/build/%s",
}

// SearchHandler2 searches the data in the new SQL backend. It times out after 3 minutes, to prevent
// outstanding requests from growing unbounded.
func (wh *Handlers) SearchHandler2(w http.ResponseWriter, r *http.Request) {
	if err := wh.limitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	q, ok := parseSearchQuery(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()
	ctx, span := trace.StartSpan(ctx, "web_SearchHandler2", trace.WithSampler(trace.AlwaysSample()))
	defer span.End()

	searchResponse, err := wh.Search2API.Search(ctx, q)
	if err != nil {
		httputils.ReportError(w, err, "Search for digests failed in the SQL backend.", http.StatusInternalServerError)
		return
	}
	sendJSONResponse(w, searchResponse)
}

// parseSearchQuery extracts the search query from request.
func parseSearchQuery(w http.ResponseWriter, r *http.Request) (*query.Search, bool) {
	q := query.Search{Limit: 50}
	if err := query.ParseSearch(r, &q); err != nil {
		httputils.ReportError(w, err, "Search for digests failed.", http.StatusInternalServerError)
		return nil, false
	}
	return &q, true
}

// DetailsHandler returns the details about a single digest.
func (wh *Handlers) DetailsHandler2(w http.ResponseWriter, r *http.Request) {
	if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}
	// TODO(kjlubick)
}

// DiffHandler returns difference between two digests.
func (wh *Handlers) DiffHandler2(w http.ResponseWriter, r *http.Request) {
	if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}
	// TODO(kjlubick)
}

// ListIgnoreRules returns the current ignore rules in JSON format.
func (wh *Handlers) ListIgnoreRules2(w http.ResponseWriter, r *http.Request) {
	if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}
	// TODO(kjlubick)
}

// UpdateIgnoreRule updates an existing ignores rule.
func (wh *Handlers) UpdateIgnoreRule2(w http.ResponseWriter, r *http.Request) {
	user := wh.loggedInAs(r)
	if user == "" {
		http.Error(w, "You must be logged in to update an ignore rule.", http.StatusUnauthorized)
		return
	}
	// TODO(kjlubick)
}

// getValidatedIgnoreRule parses the JSON from the given request into an IgnoreRuleBody. As a
// convenience, the duration as a time.Duration is returned.
func getValidatedIgnoreRule(r *http.Request) (time.Duration, frontend.IgnoreRuleBody, error) {
	irb := frontend.IgnoreRuleBody{}
	if err := parseJSON(r, &irb); err != nil {
		return 0, irb, skerr.Wrapf(err, "reading request JSON")
	}
	if irb.Filter == "" {
		return 0, irb, skerr.Fmt("must supply a filter")
	}
	// If a user accidentally includes a huge amount of text, we'd like to catch that here.
	if len(irb.Filter) >= 10*1024 {
		return 0, irb, skerr.Fmt("Filter must be < 10 KB")
	}
	if len(irb.Note) >= 1024 {
		return 0, irb, skerr.Fmt("Note must be < 1 KB")
	}
	d, err := human.ParseDuration(irb.Duration)
	if err != nil {
		return 0, irb, skerr.Wrapf(err, "invalid duration")
	}
	return d, irb, nil
}

// DeleteIgnoreRule deletes an existing ignores rule.
func (wh *Handlers) DeleteIgnoreRule2(w http.ResponseWriter, r *http.Request) {
	user := wh.loggedInAs(r)
	if user == "" {
		http.Error(w, "You must be logged in to delete an ignore rule", http.StatusUnauthorized)
		return
	}
	// TODO(kjlubick)
}

// AddIgnoreRule is for adding a new ignore rule.
func (wh *Handlers) AddIgnoreRule2(w http.ResponseWriter, r *http.Request) {
	user := wh.loggedInAs(r)
	if user == "" {
		http.Error(w, "You must be logged in to add an ignore rule", http.StatusUnauthorized)
		return
	}
	// TODO(kjlubick)
}

// TriageHandler2 handles a request to change the triage status of one or more
// digests of one test.
//
// It accepts a POST'd JSON serialization of TriageRequest and updates
// the expectations.
func (wh *Handlers) TriageHandler2(w http.ResponseWriter, r *http.Request) {
	user := login.LoggedInAs(r)
	if user == "" {
		http.Error(w, "You must be logged in to triage.", http.StatusUnauthorized)
		return
	}
	// TODO(kjlubick)
}

// StatusHandler2 returns the current status of with respect to HEAD.
func (wh *Handlers) StatusHandler2(w http.ResponseWriter, _ *http.Request) {
	// TODO(kjlubick)
}

// ClusterHandler2 calculates the NxN diffs of all the digests that match
// the incoming query and returns the data in a format appropriate for
// handling in d3.
func (wh *Handlers) ClusterHandler2(w http.ResponseWriter, r *http.Request) {
	if err := wh.limitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}
	// TODO(kjlubick)
}

// ListTestsHandler2 returns a summary of the digests seen for a given test.
func (wh *Handlers) ListTestsHandler2(w http.ResponseWriter, r *http.Request) {
	if err := wh.limitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}
	// TODO(kjlubick)
}

// TriageLogHandler2 returns the entries in the triagelog paginated
// in reverse chronological order.
func (wh *Handlers) TriageLogHandler2(w http.ResponseWriter, r *http.Request) {
	if err := wh.limitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	// TODO(kjlubick)
}

// TriageUndoHandler2 performs an "undo" for a given change id.
// The change id's are returned in the result of jsonTriageLogHandler.
// It accepts one query parameter 'id' which is the id if the change
// that should be reversed.
// If successful it returns the same result as a call to jsonTriageLogHandler
// to reflect the changed triagelog.
func (wh *Handlers) TriageUndoHandler2(w http.ResponseWriter, r *http.Request) {
	// Get the user and make sure they are logged in.
	user := login.LoggedInAs(r)
	if user == "" {
		http.Error(w, "You must be logged in to change expectations", http.StatusUnauthorized)
		return
	}
	// TODO(kjlubick)
}

// ParamsHandler2 returns all Params that could be searched over. It uses the SQL Backend
func (wh *Handlers) ParamsHandler2(w http.ResponseWriter, r *http.Request) {
	if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	if err := r.ParseForm(); err != nil {
		httputils.ReportError(w, err, "Invalid form headers", http.StatusBadRequest)
		return
	}
	clID := r.Form.Get("changelist_id")
	crs := r.Form.Get("crs")

	if clID == "" {
		ps, err := wh.Search2API.GetPrimaryBranchParamset(r.Context())
		if err != nil {
			httputils.ReportError(w, err, "Could not get paramset for primary branch", http.StatusInternalServerError)
			return
		}
		sendJSONResponse(w, ps)
		return
	}

	if _, ok := wh.getCodeReviewSystem(crs); !ok {
		http.Error(w, "Invalid Code Review System; did you include crs?", http.StatusBadRequest)
		return
	}
	ps, err := wh.Search2API.GetChangelistParamset(r.Context(), crs, clID)
	if err != nil {
		httputils.ReportError(w, err, "Could not get paramset for given CL", http.StatusInternalServerError)
		return
	}
	sendJSONResponse(w, ps)
}

// CommitsHandler returns the commits from the most recent tile.
func (wh *Handlers) CommitsHandler2(w http.ResponseWriter, r *http.Request) {
	if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}
	// TODO(kjlubick)
}

// TextKnownHashesProxy returns known hashes that have been written to GCS in the background
// Each line contains a single digest for an image. Bots will then only upload images which
// have a hash not found on this list, avoiding significant amounts of unnecessary uploads.
func (wh *Handlers) TextKnownHashesProxy(w http.ResponseWriter, r *http.Request) {
	// No limit for anon users - this is an endpoint backed up by baseline servers, and
	// should be able to handle a large load.

	w.Header().Set("Content-Type", "text/plain")
	if err := wh.GCSClient.LoadKnownDigests(r.Context(), w); err != nil {
		sklog.Errorf("Failed to copy the known hashes from GCS: %s", err)
		return
	}
}

// BaselineHandlerV2 returns a JSON representation of that baseline including
// baselines for a options issue. It can respond to requests like these:
//
//    /json/expectations
//    /json/expectations?issue=123456
//    /json/expectations?issue=123456&issueOnly=true
//
// The "issue" parameter indicates the changelist ID for which we would like to
// retrieve the baseline. In that case the returned options will be a blend of
// the master baseline and the baseline defined for the changelist (usually
// based on tryjob results).
//
// Parameter "issueOnly" is for debugging purposes only.
func (wh *Handlers) BaselineHandlerV2(w http.ResponseWriter, r *http.Request) {
	// No limit for anon users - this is an endpoint backed up by baseline servers, and
	// should be able to handle a large load.

	// TODO(kjlubick)
}

// DigestListHandler2 returns a list of digests for a given test. This is used by goldctl's
// local diff tech.
func (wh *Handlers) DigestListHandler2(w http.ResponseWriter, r *http.Request) {
	if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	// TODO(kjlubick)
}

// Whoami returns the email address of the user or service account used to authenticate the
// request. For debugging purposes only.
func (wh *Handlers) Whoami(w http.ResponseWriter, r *http.Request) {
	if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	user := wh.loggedInAs(r)
	sendJSONResponse(w, map[string]string{"whoami": user})
}

func (wh *Handlers) LatestPositiveDigestHandler(w http.ResponseWriter, r *http.Request) {
	if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}

	// TODO(kjlubick)
}

// ChangelistSearchRedirect2 redirects the user to a search page showing the search results
// for a given CL. It will do a quick scan of the untriaged digests - if it finds some, it will
// include the corpus containing some of those untriaged digests in the search query so the user
// will see results (instead of getting directed to a corpus with no results).
func (wh *Handlers) ChangelistSearchRedirect2(w http.ResponseWriter, r *http.Request) {
	if err := wh.cheapLimitForAnonUsers(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
	}

	// TODO(kjlubick)
}

func (wh *Handlers) loggedInAs(r *http.Request) string {
	if wh.testingAuthAs != "" {
		return wh.testingAuthAs
	}
	return login.LoggedInAs(r)
}

func (wh *Handlers) getCodeReviewSystem(crs string) (clstore.ReviewSystem, bool) {
	var system clstore.ReviewSystem
	found := false
	for _, rs := range wh.ReviewSystems {
		if rs.ID == crs {
			system = rs
			found = true
		}
	}
	return system, found
}

const (
	validDigestLength = 2 * md5.Size
	dotPNG            = ".png"
)

// ImageHandler returns either a single image or a diff between two images identified by their
// respective digests.
func (wh *Handlers) ImageHandler(w http.ResponseWriter, r *http.Request) {
	// No rate limit, as this should be quite fast.
	_, imgFile := path.Split(r.URL.Path)
	// Get the file that was requested and verify that it's a valid PNG file.
	if !strings.HasSuffix(imgFile, dotPNG) {
		noCacheNotFound(w, r)
		return
	}

	// Trim the image extension to get the image or diff ID.
	imgID := imgFile[:len(imgFile)-len(dotPNG)]
	// Cache images for 12 hours.
	w.Header().Set("Cache-Control", "public, max-age=43200")
	if len(imgID) == validDigestLength {
		// Example request:
		// https://skia-infra-gold.skia.org/img/images/8588cad6f3821b948468df35b67778ef.png
		wh.serveImageWithDigest(w, r, types.Digest(imgID))
	} else if len(imgID) == validDigestLength*2+1 {
		// Example request:
		// https://skia-infra-gold.skia.org/img/diffs/81c4d3a64cf32143ff6c1fbf4cbbec2d-d20731492287002a3f046eae4bd4ce7d.png
		left := types.Digest(imgID[:validDigestLength])
		// + 1 for the dash
		right := types.Digest(imgID[validDigestLength+1:])
		wh.serveImageDiff(w, r, left, right)
	} else {
		noCacheNotFound(w, r)
		return
	}
}

// serveImageWithDigest downloads the image from GCS and returns it. If there is an error, a 404
// or 500 error is returned, as appropriate.
func (wh *Handlers) serveImageWithDigest(w http.ResponseWriter, r *http.Request, digest types.Digest) {
	ctx, span := trace.StartSpan(r.Context(), "frontend_serveImageWithDigest")
	defer span.End()
	// Go's image package has no color profile support and we convert to 8-bit NRGBA to diff,
	// but our source images may have embedded color profiles and be up to 16-bit. So we must
	// at least take care to serve the original .pngs unaltered.
	b, err := wh.GCSClient.GetImage(ctx, digest)
	if err != nil {
		sklog.Warningf("Could not get image with digest %s: %s", digest, err)
		noCacheNotFound(w, r)
		return
	}
	if _, err := w.Write(b); err != nil {
		httputils.ReportError(w, err, "Could not load image. Try again later.", http.StatusInternalServerError)
		return
	}
}

// serveImageDiff downloads the left and right images, computes the diff between them, encodes
// the diff as a PNG image and writes it to the provided ResponseWriter. If there is an error, it
// returns a 404 or 500 error as appropriate.
func (wh *Handlers) serveImageDiff(w http.ResponseWriter, r *http.Request, left types.Digest, right types.Digest) {
	ctx, span := trace.StartSpan(r.Context(), "frontend_serveImageDiff")
	defer span.End()
	// TODO(lovisolo): Diff in NRGBA64?
	// TODO(lovisolo): Make sure each pair of images is in the same color space before diffing?
	//                 (They probably are today but it'd be a good correctness check to make sure.)
	eg, eCtx := errgroup.WithContext(ctx)
	var leftImg *image.NRGBA
	var rightImg *image.NRGBA
	eg.Go(func() error {
		b, err := wh.GCSClient.GetImage(eCtx, left)
		if err != nil {
			return skerr.Wrap(err)
		}
		leftImg, err = decode(b)
		return skerr.Wrap(err)
	})
	eg.Go(func() error {
		b, err := wh.GCSClient.GetImage(eCtx, right)
		if err != nil {
			return skerr.Wrap(err)
		}
		rightImg, err = decode(b)
		return skerr.Wrap(err)
	})
	if err := eg.Wait(); err != nil {
		sklog.Warningf("Could not get diff for images %q and %q: %s", left, right, err)
		noCacheNotFound(w, r)
		return
	}
	// Compute the diff image.
	_, diffImg := diff.PixelDiff(leftImg, rightImg)

	// Write output image to the http.ResponseWriter. Content-Type is set automatically
	// based on the first 512 bytes of written data. See docs for ResponseWriter.Write()
	// for details.
	//
	// The encoding step below does not take color profiles into account. This is fine since
	// both the left and right images used to compute the diff are in the same color space,
	// and also because the resulting diff image is just a visual approximation of the
	// differences between the left and right images.
	if err := encodeImg(w, diffImg); err != nil {
		httputils.ReportError(w, err, "could not serve diff image", http.StatusInternalServerError)
		return
	}
}

// decode decodes the provided bytes as a PNG and returns them as an *image.NRGBA.
func decode(b []byte) (*image.NRGBA, error) {
	im, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return diff.GetNRGBA(im), nil
}

// noCacheNotFound disables caching and returns a 404.
func noCacheNotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	http.NotFound(w, r)
}

// ChangelistSummaryHandler returns a summary of the new and untriaged digests produced by this
// CL across all Patchsets.
func (wh *Handlers) ChangelistSummaryHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "web_ChangelistSummaryHandler")
	defer span.End()
	if err := wh.cheapLimitForGerritPlugin(r); err != nil {
		httputils.ReportError(w, err, "Try again later", http.StatusInternalServerError)
		return
	}
	clID, ok := mux.Vars(r)["id"]
	if !ok {
		http.Error(w, "Must specify 'id' of Changelist.", http.StatusBadRequest)
		return
	}
	crs, ok := mux.Vars(r)["system"]
	if !ok {
		http.Error(w, "Must specify 'system' of Changelist.", http.StatusBadRequest)
		return
	}
	system, ok := wh.getCodeReviewSystem(crs)
	if !ok {
		http.Error(w, "Invalid Code Review System", http.StatusBadRequest)
		return
	}

	qCLID := sql.Qualify(system.ID, clID)
	sum, err := wh.getCLSummary2(ctx, qCLID)
	if err != nil {
		httputils.ReportError(w, err, "Could not get summary", http.StatusInternalServerError)
		return
	}
	rv := search2_fe.ConvertChangelistSummaryResponseV1(sum)
	sendJSONResponse(w, rv)
}

// getCLSummary2 fetches, caches, and returns the summary for a given CL. If the result has already
// been cached, it will return that cached value with a flag if the value is still up to date or
// not. If the cached data is stale, it will spawn a goroutine to update the cached value.
func (wh *Handlers) getCLSummary2(ctx context.Context, qCLID string) (search2.NewAndUntriagedSummary, error) {
	ts, err := wh.Search2API.ChangelistLastUpdated(ctx, qCLID)
	if err != nil {
		return search2.NewAndUntriagedSummary{}, skerr.Wrap(err)
	}
	if ts.IsZero() { // A Zero time means we have no data for this CL.
		return search2.NewAndUntriagedSummary{}, nil
	}

	cached, ok := wh.clSummaryCache.Get(qCLID)
	if ok {
		sum, ok := cached.(search2.NewAndUntriagedSummary)
		if ok {
			if ts.Before(sum.LastUpdated) || sum.LastUpdated.Equal(ts) {
				sum.Outdated = false
				return sum, nil
			}
			// Result is stale. Start a goroutine to fetch it again.
			done := make(chan struct{})
			go func() {
				// We intentionally use context.Background() and not the request's context because
				// if we return a result, we want the fetching in the background to continue so
				// if/when the client tries again, we can serve that updated result.
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				newValue, err := wh.Search2API.NewAndUntriagedSummaryForCL(ctx, qCLID)
				if err != nil {
					sklog.Warningf("Could not fetch out of date summary for cl %s in background: %s", qCLID, err)
					return
				}
				wh.clSummaryCache.Add(qCLID, newValue)
				done <- struct{}{}
			}()
			// Wait up to 500ms to return the latest value quickly if available
			timer := time.NewTimer(500 * time.Millisecond)
			defer timer.Stop()
			select {
			case <-done:
			case <-timer.C:
			}
			cached, ok = wh.clSummaryCache.Get(qCLID)
			if ok {
				if possiblyUpdated, ok := cached.(search2.NewAndUntriagedSummary); ok {
					if ts.Before(possiblyUpdated.LastUpdated) || possiblyUpdated.LastUpdated.Equal(ts) {
						// We were able to fetch new data quickly, so return it now.
						possiblyUpdated.Outdated = false
						return possiblyUpdated, nil
					}
				}
			}
			// The cached data is still stale or invalid, so return what we have marked as outdated.
			sum.Outdated = true
			return sum, nil
		}
	}
	// Invalid or missing cache entry. We must fetch because we have nothing to give the user.
	sum, err := wh.Search2API.NewAndUntriagedSummaryForCL(ctx, qCLID)
	if err != nil {
		return search2.NewAndUntriagedSummary{}, skerr.Wrap(err)
	}
	wh.clSummaryCache.Add(qCLID, sum)
	return sum, nil
}

// StartCacheWarming starts a go routine to warm the CL Summary cache. This way, most summaries are
// responsive, even on big instances.
func (wh *Handlers) StartCacheWarming(ctx context.Context) {
	// We warm every CL that was open and produced data or saw triage activity in the last 5 days.
	// After the first cycle, we will incrementally update the cache.
	lastCheck := now.Now(ctx).Add(-5 * 24 * time.Hour)
	go util.RepeatCtx(ctx, time.Minute, func(ctx context.Context) {
		ctx, span := trace.StartSpan(ctx, "web_warmCacheCycle", trace.WithSampler(trace.AlwaysSample()))
		defer span.End()
		newTS := now.Now(ctx)
		rows, err := wh.DB.Query(ctx, `WITH
ChangelistsWithNewData AS (
	SELECT changelist_id FROM Changelists
	WHERE status = 'open' and last_ingested_data > $1
),
ChangelistsWithTriageActivity AS (
	SELECT DISTINCT branch_name AS changelist_id FROM ExpectationRecords
	WHERE branch_name IS NOT NULL AND triage_time > $1
)
SELECT changelist_id FROM ChangelistsWithNewData
UNION
SELECT changelist_id FROM ChangelistsWithTriageActivity
`, lastCheck)
		if err != nil {
			if err == pgx.ErrNoRows {
				sklog.Infof("No CLS updated since %s", lastCheck)
				lastCheck = newTS
				return
			}
			sklog.Errorf("Could not fetch updated CLs to warm cache: %s", err)
			return
		}
		defer rows.Close()
		var qualifiedIDS []string
		for rows.Next() {
			var qID string
			if err := rows.Scan(&qID); err != nil {
				sklog.Errorf("Could not scan: %s", err)
			}
			qualifiedIDS = append(qualifiedIDS, qID)
		}
		sklog.Infof("Warming cache for %d CLs", len(qualifiedIDS))
		span.AddAttributes(trace.Int64Attribute("num_cls", int64(len(qualifiedIDS))))
		// warm cache 3 at a time. This number of goroutines was chosen arbitrarily.
		_ = util.ChunkIterParallel(ctx, len(qualifiedIDS), len(qualifiedIDS)/3+1, func(ctx context.Context, startIdx int, endIdx int) error {
			if err := ctx.Err(); err != nil {
				return nil
			}
			for _, qCLID := range qualifiedIDS[startIdx:endIdx] {
				_, err := wh.getCLSummary2(ctx, qCLID)
				if err != nil {
					sklog.Warningf("Ignoring error while warming CL Cache for %s: %s", qCLID, err)
				}
			}
			return nil
		})
		lastCheck = newTS
		sklog.Infof("Done warming cache")
	})
}
