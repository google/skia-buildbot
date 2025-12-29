// Package frame takes frontend requests for dataframes (FrameRequest), and
// turns them into FrameResponses.
package frame

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strings"
	"time"

	"go.opencensus.io/trace"
	"go.skia.org/infra/go/calc"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/anomalies"
	"go.skia.org/infra/perf/go/chromeperf"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/pivot"
	"go.skia.org/infra/perf/go/progress"
	"go.skia.org/infra/perf/go/shortcut"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/types"
)

// RequestType distinguishes the domain of the traces returned in a
// FrameResponse.
type RequestType int

const (
	// Values for FrameRequest.RequestType.
	REQUEST_TIME_RANGE RequestType = 0
	REQUEST_COMPACT    RequestType = 1

	DEFAULT_COMPACT_NUM_COMMITS = 200
)

// AllRequestType is all possible values for a RequestType variable.
var AllRequestType = []RequestType{REQUEST_TIME_RANGE, REQUEST_COMPACT}

const (
	maxTracesInResponse = 350
)

// ResponseDisplayMode are the different modes of the explore-sk page.
type ResponseDisplayMode string

const (
	// DisplayQueryOnly means just display the Query button.
	DisplayQueryOnly ResponseDisplayMode = "display_query_only"

	// DisplayPlot display the results of a query as a plot.
	DisplayPlot ResponseDisplayMode = "display_plot"

	// DisplayPivotTable display the results of a query as a pivot table.
	DisplayPivotTable ResponseDisplayMode = "display_pivot_table"

	// DisplayPivotPlot display the results of a query as a plot of pivoted traces.
	DisplayPivotPlot ResponseDisplayMode = "display_pivot_plot"

	// DisplaySpinner display the spinner indicating we are waiting for results.
	DisplaySpinner ResponseDisplayMode = "display_spinner"
)

// AllResponseDisplayModes lists all ResponseDisplayMode for use by go2ts.
var AllResponseDisplayModes = []ResponseDisplayMode{
	DisplayQueryOnly,
	DisplayPlot,
	DisplayPivotTable,
	DisplayPivotPlot,
	DisplaySpinner,
}

// FrameRequest is used to deserialize JSON frame requests.
type FrameRequest struct {
	Begin                   int         `json:"begin"`                 // Beginning of time range in Unix timestamp seconds.
	End                     int         `json:"end"`                   // End of time range in Unix timestamp seconds.
	Formulas                []string    `json:"formulas,omitempty"`    // The Formulae to evaluate.
	Queries                 []string    `json:"queries,omitempty"`     // The queries to perform encoded as a URL query.
	Keys                    string      `json:"keys,omitempty"`        // The id of a list of keys stored via shortcut2.
	TZ                      string      `json:"tz"`                    // The timezone the request is from. https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/DateTimeFormat/resolvedOptions
	NumCommits              int32       `json:"num_commits,omitempty"` // If RequestType is REQUEST_COMPACT, then the number of commits to show before End, and Begin is ignored.
	RequestType             RequestType `json:"request_type,omitempty"`
	DoNotFilterParentTraces bool        `json:"disable_filter_parent_traces,omitempty"`

	Pivot *pivot.Request `json:"pivot,omitempty"`

	Progress progress.Progress `json:"-"`
}

// NewFrameRequest returns a new FrameRequest instance.
func NewFrameRequest() *FrameRequest {
	return &FrameRequest{
		Progress: progress.New(),
	}
}

// FrameResponse is serialized to JSON as the response to frame requests.
type FrameResponse struct {
	DataFrame   *dataframe.DataFrame  `json:"dataframe"`
	Skps        []int                 `json:"skps"`
	Msg         string                `json:"msg"`
	DisplayMode ResponseDisplayMode   `json:"display_mode"`
	AnomalyMap  chromeperf.AnomalyMap `json:"anomalymap"`
}

// frameRequestProcess keeps track of a running Go routine that's
// processing a FrameRequest to build a FrameResponse.
type frameRequestProcess struct {
	// request is read-only, it should not be modified.
	request *FrameRequest

	perfGit perfgit.Git

	// dfBuilder builds DataFrame's.
	dfBuilder dataframe.DataFrameBuilder

	shortcutStore shortcut.Store

	search        int     // The current search (either Formula or Query) being processed.
	totalSearches int     // The total number of Formulas and Queries in the FrameRequest.
	percent       float32 // The percentage of the searches complete [0.0-1.0].
}

// ProcessFrameRequest starts processing a FrameRequest.
//
// It does not return until all the work is complete.
//
// The finished results are stored in the FrameRequestProcess.Progress.Results.
func ProcessFrameRequest(ctx context.Context, req *FrameRequest, perfGit perfgit.Git, dfBuilder dataframe.DataFrameBuilder, traceStore tracestore.TraceStore, metadataStore tracestore.MetadataStore, shortcutStore shortcut.Store, anomalyStore anomalies.Store, searchAnomaliesTimeBased bool) error {
	numKeys := 0
	if req.Keys != "" {
		numKeys = 1
	}
	ret := &frameRequestProcess{
		perfGit:       perfGit,
		request:       req,
		totalSearches: len(req.Formulas) + len(req.Queries) + numKeys,
		dfBuilder:     dfBuilder,
		shortcutStore: shortcutStore,
	}
	df, err := ret.run(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}

	// Do not truncate pivot requests.
	truncate := req.Pivot == nil || req.Pivot.Valid() != nil
	resp, err := ResponseFromDataFrame(ctx, req.Pivot, df, ret.perfGit, truncate, ret.request.Progress)
	if err != nil {
		return ret.reportError(err, "Failed to get skps.")
	}

	if metadataStore != nil {
		// Get the metadata for the retrieved traces.
		traceMetadata, err := getMetadataForTraces(ctx, df, metadataStore)
		if err != nil {
			return ret.reportError(err, "Failed to get trace metadata.")
		}

		// Attach the metadata to the dataframe.
		resp.DataFrame.TraceMetadata = traceMetadata
	}

	if searchAnomaliesTimeBased {
		addTimeBasedAnomaliesToResponse(ctx, resp, anomalyStore, ret.perfGit)
	} else {
		addRevisionBasedAnomaliesToResponse(ctx, resp, anomalyStore, ret.perfGit)
	}
	sklog.Debugf("Resp has AnomalyMap: %v", resp.AnomalyMap)

	ret.request.Progress.Results(resp)
	return nil

}

// reportError records the reason a FrameRequestProcess failed.
func (p *frameRequestProcess) reportError(err error, message string) error {
	sklog.Errorf("FrameRequest failed: %#v %s: %s", *(p.request), message, err)
	return skerr.Wrapf(err, "%s", message)
}

// searchInc records the progress of a FrameRequestProcess as it completes each
// Query or Formula.
func (p *frameRequestProcess) searchInc() {
	p.search += 1
}

// run does the work in a FrameRequestProcess. It does not return until all the
// work is done or the request failed. Should be run as a Go routine.
func (p *frameRequestProcess) run(ctx context.Context) (*dataframe.DataFrame, error) {
	ctx, span := trace.StartSpan(ctx, "FrameRequestProcess.Run")
	defer span.End()

	begin := time.Unix(int64(p.request.Begin), 0).UTC()
	end := time.Unix(int64(p.request.End), 0).UTC()

	// Results from all the queries and calcs will be accumulated in this dataframe.
	df := dataframe.NewEmpty()

	p.request.Progress.Message("Loading", "Queries")
	// Queries.
	for _, q := range p.request.Queries {
		newDF, err := p.doSearch(ctx, q, begin, end)
		if err != nil {
			return nil, p.reportError(err, "Failed to complete query for search.")
		}
		df = dataframe.Join(df, newDF)
		p.searchInc()
	}

	p.request.Progress.Message("Loading", "Formulas")

	// Formulas.
	for _, formula := range p.request.Formulas {
		newDF, err := p.doCalc(ctx, formula, begin, end)
		if err != nil {
			return nil, p.reportError(err, "Failed to complete query for calculations")
		}
		df = dataframe.Join(df, newDF)
		p.searchInc()
	}

	p.request.Progress.Message("Loading", "Keys")

	// Keys
	if p.request.Keys != "" {
		newDF, err := p.doKeys(ctx, p.request.Keys, begin, end)
		if err != nil {
			return nil, p.reportError(err, "Failed to complete query for keys")
		}
		df = dataframe.Join(df, newDF)
	}

	p.request.Progress.Message("Loading", "Finished")

	if len(df.Header) == 0 {
		var err error
		df, err = dataframe.NewHeaderOnly(ctx, p.perfGit, begin, end)
		if err != nil {
			return nil, p.reportError(err, "Failed to load dataframe.")
		}
	}

	// Pivot
	if p.request.Pivot != nil && len(p.request.Pivot.GroupBy) > 0 {
		var err error
		df, err = pivot.Pivot(ctx, *p.request.Pivot, df)
		if err != nil {
			return nil, p.reportError(err, "Pivot failed.")
		}
	}

	return df, nil
}

// getSkps returns the indices where the SKPs have been updated given
// the ColumnHeaders.
//
// TODO(jcgregorio) Rename this functionality to something more generic.
func getSkps(ctx context.Context, headers []*dataframe.ColumnHeader, perfGit perfgit.Git) ([]int, error) {
	if perfGit == nil {
		return []int{}, nil
	}
	if config.Config == nil || config.Config.GitRepoConfig.FileChangeMarker == "" {
		return []int{}, nil
	}
	begin := types.CommitNumber(headers[0].Offset)
	end := types.CommitNumber(headers[len(headers)-1].Offset)

	commitNumbers, err := perfGit.CommitNumbersWhenFileChangesInCommitNumberRange(ctx, begin, end, config.Config.GitRepoConfig.FileChangeMarker)
	if err != nil {
		return []int{}, skerr.Wrapf(err, "Failed to find skp changes for range: %d-%d", begin, end)
	}
	ret := make([]int, len(commitNumbers))
	for i, n := range commitNumbers {
		ret[i] = int(n)
	}

	return ret, nil
}

// ResponseFromDataFrame fills out the rest of a FrameResponse for the given DataFrame.
//
// If truncate is true then the number of traces returned is limited.
//
// tz is the timezone, and can be the empty string if the default (Eastern) timezone is acceptable.
func ResponseFromDataFrame(ctx context.Context, pivotRequest *pivot.Request, df *dataframe.DataFrame, perfGit perfgit.Git, truncate bool, progress progress.Progress) (*FrameResponse, error) {
	if len(df.Header) == 0 {
		return nil, fmt.Errorf("No commits matched that time range.")
	}

	// Determine where SKP changes occurred.
	skps, err := getSkps(ctx, df.Header, perfGit)
	if err != nil {
		sklog.Errorf("Failed to load skps: %s", err)
	}

	// Truncate the result if it's too large.
	if truncate && len(df.TraceSet) > maxTracesInResponse {
		progress.Message("Message", fmt.Sprintf("Response too large, the number of traces returned has been truncated from %d to %d.", len(df.TraceSet), maxTracesInResponse))
		keys := []string{}
		for k := range df.TraceSet {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		keys = keys[:maxTracesInResponse]
		newTraceSet := types.TraceSet{}
		for _, key := range keys {
			newTraceSet[key] = df.TraceSet[key]
		}
		df.TraceSet = newTraceSet
	}

	// Determine the DisplayMode to return.
	displayMode := DisplayPlot
	if pivotRequest != nil && len(pivotRequest.GroupBy) > 0 {
		displayMode = DisplayPivotPlot
		if len(pivotRequest.Summary) > 0 {
			displayMode = DisplayPivotTable
		}
	}

	return &FrameResponse{
		DataFrame:   df,
		Skps:        skps,
		DisplayMode: displayMode,
	}, nil
}

// getMetadataForTraces returns the metadata for the traces present in the data frame for the relevant commits.
func getMetadataForTraces(ctx context.Context, df *dataframe.DataFrame, metadataStore tracestore.MetadataStore) ([]types.TraceMetadata, error) {
	// Extract the relevant commit numbers and trace ids from the data frame.
	commitNumbers := make([]types.CommitNumber, len(df.Header))
	traceIds := []string{}
	sourceFileIds := []int64{}
	for i := 0; i < len(df.Header); i++ {
		commitNumbers[i] = df.Header[i].Offset
	}
	for traceId := range df.TraceSet {
		traceIds = append(traceIds, traceId)
	}
	if len(traceIds) == 0 {
		return []types.TraceMetadata{}, nil
	}

	// Collect all the source file ids for which we will extract metadata.
	foundIds := map[int64]bool{}
	for _, traceId := range traceIds {
		sourceMap := df.SourceInfo[traceId]
		if sourceMap != nil {
			for _, sourceFileId := range sourceMap.GetAllSourceFileIds() {
				foundIds[sourceFileId] = true
			}
		}
	}
	for sourceFileId := range foundIds {
		sourceFileIds = append(sourceFileIds, sourceFileId)
	}

	if len(sourceFileIds) == 0 {
		return []types.TraceMetadata{}, nil
	}

	// The return value is a map where the key is the source file name and value is the map of links.
	linkInfo, err := metadataStore.GetMetadataForSourceFileIDs(ctx, sourceFileIds)
	if err != nil {
		return nil, skerr.Wrapf(err, "Error getting metadata")
	}

	traceMetadatas := []types.TraceMetadata{}
	// rawLinks is a nested map that tracks links from database for a
	// traceId->commitnum->links lookup.
	rawLinks := map[string]map[types.CommitNumber]map[string]string{}
	for traceId := range df.TraceSet {
		traceMetadata := types.TraceMetadata{
			TraceID:     traceId,
			CommitLinks: map[types.CommitNumber]map[string]types.TraceCommitLink{},
		}

		sourceInfo := df.SourceInfo[traceId]
		if sourceInfo != nil {
			// Populate the raw links for this trace for the relevant commit numbers.
			rawLinks[traceId] = map[types.CommitNumber]map[string]string{}
			for _, commitnum := range sourceInfo.GetAllCommitNumbers() {
				sourceFileName, ok := sourceInfo.Get(commitnum)
				if !ok {
					continue
				}
				links := linkInfo[sourceFileName]
				rawLinks[traceId][commitnum] = links
			}
		}
		traceMetadatas = append(traceMetadatas, traceMetadata)
	}

	// Now that we have the metadata and the raw links, let's populate the commit links in
	// the metadata objects based on instance configuration.
	return populateTraceMetadataLinksBasedOnConfig(traceMetadatas, df.Header[0].Offset, rawLinks), nil
}

// populateTraceMetadataLinksBasedOnConfig populates the commit links for the trace metadata
// objects based on the specifications in the instance configs.
//
// Links which are commits are modified to show commit ranges from prev data points for well known keys.
// Links which are specified as useful in the instance config are added from the raw links.
// All other links are ignored.
func populateTraceMetadataLinksBasedOnConfig(traceCommitLinks []types.TraceMetadata, startCommitNumber types.CommitNumber, rawLinks map[string]map[types.CommitNumber]map[string]string) []types.TraceMetadata {
	ret := []types.TraceMetadata{}
	for _, traceCommitLink := range traceCommitLinks {
		for currentCommit := range rawLinks[traceCommitLink.TraceID] {
			traceCommitLink.CommitLinks[currentCommit] = map[string]types.TraceCommitLink{}

			// Let's find the prev commit for this trace that has data.
			prevCommit := currentCommit - 1
			for prevCommit >= startCommitNumber {
				if _, ok := rawLinks[traceCommitLink.TraceID][prevCommit]; !ok {
					// if no data present, we move on.
					prevCommit = prevCommit - 1
				} else {
					break
				}
			}
			if prevCommit >= startCommitNumber {
				// Define a func that will easily extract the commit id from a commit url.
				getCommitIdFromCommitUrl := func(commitUrl string) string {
					commitIdStartIdx := strings.LastIndex(commitUrl, "/")
					return commitUrl[commitIdStartIdx+1:]
				}
				// Define a func that will easily extract the repo url from the commit url.
				getRepoUrlFromCommitUrl := func(commitUrl string) string {
					repoEndIdx := strings.Index(commitUrl, "+")
					return commitUrl[:repoEndIdx]
				}
				currentCommitLinks := rawLinks[traceCommitLink.TraceID][currentCommit]
				prevCommitLinks := rawLinks[traceCommitLink.TraceID][prevCommit]

				// Let's assemble the commit range urls if specified in the config.
				if len(config.Config.DataPointConfig.KeysForCommitRange) > 0 {
					for _, key := range config.Config.DataPointConfig.KeysForCommitRange {
						currentCommitidInLinks := getCommitIdFromCommitUrl(currentCommitLinks[key])
						if currentCommitidInLinks != "" {
							prevCommitidInLinks := getCommitIdFromCommitUrl(prevCommitLinks[key])
							switch key {
							case "V8":
								currentCommitLinks[key] = "https://chromium.googlesource.com/v8/v8/+/" + currentCommitidInLinks
								if prevCommitLinks != nil {
									prevCommitLinks[key] = "https://chromium.googlesource.com/v8/v8/+/" + prevCommitidInLinks
								}
							case "WebRTC":
								currentCommitLinks[key] = "https://chromium.googlesource.com/external/webrtc/+/" + currentCommitidInLinks
								if prevCommitLinks != nil {
									prevCommitLinks[key] = "https://chromium.googlesource.com/external/webrtc/+/" + prevCommitidInLinks
								}
							}
							var linkValue types.TraceCommitLink
							// There is no relevant change in the commits specified in the links.
							if currentCommitidInLinks == prevCommitidInLinks || prevCommitidInLinks == "" {
								linkValue = types.TraceCommitLink{
									Text: fmt.Sprintf("%s (No Change)", currentCommitidInLinks[:8]),
									Href: currentCommitLinks[key],
								}
							} else {
								// Since the prev commit and current commit are different, show is as a commit range url.
								commitRangeUrl := fmt.Sprintf("%s+log/%s..%s", getRepoUrlFromCommitUrl(currentCommitLinks[key]), prevCommitidInLinks, currentCommitidInLinks)
								linkValue = types.TraceCommitLink{
									Text: fmt.Sprintf("%s - %s", prevCommitidInLinks[:8], currentCommitidInLinks[:8]),
									Href: commitRangeUrl,
								}
							}
							traceCommitLink.CommitLinks[currentCommit][key] = linkValue
						}
					}
				}

				// Let's assemble the other useful links if specified in the config.
				if len(config.Config.DataPointConfig.KeysForUsefulLinks) > 0 {
					for key, link := range currentCommitLinks {
						if slices.Contains(config.Config.DataPointConfig.KeysForUsefulLinks, key) {
							traceCommitLink.CommitLinks[currentCommit][key] = types.TraceCommitLink{
								Href: link,
							}
						}
					}
				}
			}
		}
		ret = append(ret, traceCommitLink)
	}

	return ret
}

func addTimeBasedAnomaliesToResponse(ctx context.Context, response *FrameResponse, anomalyStore anomalies.Store, perfGit perfgit.Git) {
	ctx, span := trace.StartSpan(ctx, "addTimeBasedAnomaliesToResponse")
	defer span.End()
	df := response.DataFrame
	if anomalyStore != nil && df != nil && len(df.TraceSet) > 0 {
		startCommitPosition := df.Header[0].Offset
		endCommitPosition := df.Header[len(df.Header)-1].Offset
		traceNames := make([]string, 0)
		for traceName := range df.TraceSet {
			traceNames = append(traceNames, traceName)
		}

		startCommit, err := perfGit.CommitFromCommitNumber(ctx, types.CommitNumber(startCommitPosition))
		if err != nil {
			sklog.Errorf("Unable to get commit details for commit position %d", startCommitPosition)
			return
		}
		endCommit, err := perfGit.CommitFromCommitNumber(ctx, types.CommitNumber(endCommitPosition))
		if err != nil {
			sklog.Errorf("Unable to get commit details for commit position %d", endCommitPosition)
			return
		}

		startTime := time.Unix(startCommit.Timestamp, 0)
		// Add 2 days to the end time since a perf run may happen a bit later after the
		// commit and the anomaly is generated only after a perf run is done, if at all.
		endTime := time.Unix(endCommit.Timestamp, 0).Add(time.Duration(48) * time.Hour)
		// Fetch Chrome Perf anomalies.
		anomalyMap, err := anomalyStore.GetAnomaliesInTimeRange(ctx, traceNames, startTime, endTime)
		if err != nil {
			// Won't fail the frame request if there was error while fetching the Chrome Perf anomaly,
			sklog.Errorf("Failed to fetch anomalies from anomaly store. %s", err)
		}

		// Attach anomaly map to DataFrame
		response.AnomalyMap = anomalyMap
	}
}

// addRevisionBasedAnomaliesToResponse fetch Chrome Perf anomalies and attach them to the response.
func addRevisionBasedAnomaliesToResponse(ctx context.Context, response *FrameResponse, anomalyStore anomalies.Store, perfGit perfgit.Git) {
	ctx, span := trace.StartSpan(ctx, "addRevisionBasedAnomaliesToResponse")
	defer span.End()
	df := response.DataFrame
	if anomalyStore != nil && df != nil && len(df.TraceSet) > 0 {
		startCommitPosition := df.Header[0].Offset
		endCommitPosition := df.Header[len(df.Header)-1].Offset
		traceNames := make([]string, 0)
		for traceName := range df.TraceSet {
			traceNames = append(traceNames, traceName)
		}

		// Fetch Chrome Perf anomalies.
		anomalyMap, err := anomalyStore.GetAnomalies(ctx, traceNames, int(startCommitPosition), int(endCommitPosition))
		if err != nil {
			// Won't fail the frame request if there was error while fetching the Chrome Perf anomaly,
			sklog.Errorf("Failed to fetch anomalies from anomaly store. %s", err)
		}

		// Attach anomaly map to DataFrame
		response.AnomalyMap = anomalyMap
	}
}

// doSearch applies the given query and returns a dataframe that matches the
// given time range [begin, end) in a DataFrame.
func (p *frameRequestProcess) doSearch(ctx context.Context, queryStr string, begin, end time.Time) (*dataframe.DataFrame, error) {
	ctx, span := trace.StartSpan(ctx, "FrameRequestProcess.doSearch")
	defer span.End()

	urlValues, err := url.ParseQuery(queryStr)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse query: %s", err)
	}
	q, err := query.New(urlValues)
	if err != nil {
		return nil, fmt.Errorf("Invalid Query: %s", err)
	}
	p.request.Progress.Message("Query", q.String())
	if p.request.RequestType == REQUEST_TIME_RANGE {
		return p.dfBuilder.NewFromQueryAndRange(ctx, begin, end, q, p.request.Progress)
	}
	return p.dfBuilder.NewNFromQuery(ctx, end, q, p.request.NumCommits, p.request.Progress)

}

// doKeys returns a DataFrame that matches the given set of keys given
// the time range [begin, end).
func (p *frameRequestProcess) doKeys(ctx context.Context, keyID string, begin, end time.Time) (*dataframe.DataFrame, error) {
	keys, err := p.shortcutStore.Get(ctx, keyID)
	if err != nil {
		return nil, fmt.Errorf("Failed to find that set of keys %q: %s", keyID, err)
	}
	if p.request.RequestType == REQUEST_TIME_RANGE {
		return p.dfBuilder.NewFromKeysAndRange(ctx, keys.Keys, begin, end, p.request.Progress)
	}
	return p.dfBuilder.NewNFromKeys(ctx, end, keys.Keys, p.request.NumCommits, p.request.Progress)

}

// doCalc applies the given formula and returns a dataframe that matches the
// given time range [begin, end) in a DataFrame.
func (p *frameRequestProcess) doCalc(ctx context.Context, formula string, begin, end time.Time) (*dataframe.DataFrame, error) {
	// During the calculation 'rowsFromQuery' will be called to load up data, we
	// will capture the dataframe that's created at that time. We only really
	// need df.Headers so it doesn't matter if the calculation has multiple calls
	// to filter(), we can just use the last one returned.
	var df *dataframe.DataFrame

	rowsFromQuery := func(s string) (types.TraceSet, error) {
		urlValues, err := url.ParseQuery(s)
		if err != nil {
			return nil, err
		}
		q, err := query.New(urlValues)
		if err != nil {
			return nil, err
		}
		if p.request.RequestType == REQUEST_TIME_RANGE {
			df, err = p.dfBuilder.NewFromQueryAndRange(ctx, begin, end, q, p.request.Progress)
		} else {
			df, err = p.dfBuilder.NewNFromQuery(ctx, end, q, p.request.NumCommits, p.request.Progress)
		}
		if err != nil {
			return nil, err
		}
		// DataFrames are float32, but calc does its work in float64.
		rows := types.TraceSet{}
		for k, v := range df.TraceSet {
			rows[k] = vec32.Dup(v)
		}
		return rows, nil
	}

	rowsFromShortcut := func(s string) (types.TraceSet, error) {
		keys, err := p.shortcutStore.Get(ctx, s)
		if err != nil {
			return nil, err
		}
		if p.request.RequestType == REQUEST_TIME_RANGE {
			df, err = p.dfBuilder.NewFromKeysAndRange(ctx, keys.Keys, begin, end, p.request.Progress)
		} else {
			df, err = p.dfBuilder.NewNFromKeys(ctx, end, keys.Keys, p.request.NumCommits, p.request.Progress)
		}
		if err != nil {
			return nil, err
		}
		// DataFrames are float32, but calc does its work in float64.
		rows := types.TraceSet{}
		for k, v := range df.TraceSet {
			rows[k] = vec32.Dup(v)
		}
		return rows, nil
	}

	calcContext := calc.NewContext(rowsFromQuery, rowsFromShortcut)
	rows, err := calcContext.Eval(formula)
	if err != nil {
		return nil, skerr.Wrapf(err, "Calculation failed")
	}

	// Convert the Rows from float64 to float32 for DataFrame.
	ts := types.TraceSet{}
	for k, v := range rows {
		ts[k] = v
	}
	df.TraceSet = ts

	// Clear the paramset since we are returning calculated values.
	df.ParamSet = paramtools.NewReadOnlyParamSet()

	return df, nil
}
