// Package continuous looks for Regressions in the background based on the
// new data arriving and the currently configured Alerts.
package continuous

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/ctxutil"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/pubsub/sub"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/ingestevents"
	"go.skia.org/infra/perf/go/notify"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/shortcut"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/urlprovider"
)

const (
	// maxParallelReceives is the maximum number of Go routines used when
	// receiving PubSub messages.
	maxParallelReceives = 1

	// pollingClusteringDelay is the time to wait between clustering runs, but
	// only when not doing event driven regression detection.
	pollingClusteringDelay = 5 * time.Second

	doNotOverrideQuery = ""

	// This is the no of parallel goroutines that will process the traces for a
	// given alert config.
	processAlertConfigForTracesWorkerCount = 5

	// This is the no of parallel goroutines that will process alert configs for
	// the incoming event.
	processAlertConfigsWorkerCount = 20

	timeoutForProcessAlertConfigPerTrace time.Duration = time.Minute
)

// Continuous is used to run clustering on the last numCommits commits and
// look for regressions.
type Continuous struct {
	perfGit        perfgit.Git
	shortcutStore  shortcut.Store
	store          regression.Store
	provider       alerts.ConfigProvider
	notifier       notify.Notifier
	paramsProvider regression.ParamsetProvider
	urlProvider    urlprovider.URLProvider
	dfBuilder      dataframe.DataFrameBuilder
	pollingDelay   time.Duration
	instanceConfig *config.InstanceConfig
	flags          *config.FrontendFlags

	mutex             sync.Mutex // Protects current.
	current           *alerts.Alert
	regressionCounter metrics2.Counter
}

// New creates a new *Continuous.
//
//	provider - Produces the slice of alerts.Config's that determine the clustering to perform.
//	numCommits - The number of commits to run the clustering over.
//	radius - The number of commits on each side of a commit to include when clustering.
func New(
	perfGit perfgit.Git,
	shortcutStore shortcut.Store,
	provider alerts.ConfigProvider,
	store regression.Store,
	notifier notify.Notifier,
	paramsProvider regression.ParamsetProvider,
	urlProvider urlprovider.URLProvider,
	dfBuilder dataframe.DataFrameBuilder,
	instanceConfig *config.InstanceConfig,
	flags *config.FrontendFlags) *Continuous {
	return &Continuous{
		perfGit:           perfGit,
		store:             store,
		provider:          provider,
		notifier:          notifier,
		shortcutStore:     shortcutStore,
		current:           &alerts.Alert{},
		paramsProvider:    paramsProvider,
		urlProvider:       urlProvider,
		dfBuilder:         dfBuilder,
		pollingDelay:      pollingClusteringDelay,
		instanceConfig:    instanceConfig,
		flags:             flags,
		regressionCounter: metrics2.GetCounter("continuous_regression_found"),
	}
}

func (c *Continuous) reportRegressions(ctx context.Context, req *regression.RegressionDetectionRequest, resps []*regression.RegressionDetectionResponse, cfg *alerts.Alert) {
	ctx, span := trace.StartSpan(ctx, "regression.continuous.reportRegressions")
	defer span.End()

	key := cfg.IDAsString
	for _, resp := range resps {
		headerLength := len(resp.Frame.DataFrame.Header)
		midPoint := headerLength / 2
		commitNumber := resp.Frame.DataFrame.Header[midPoint].Offset
		details, err := c.perfGit.CommitFromCommitNumber(ctx, commitNumber)
		if err != nil {
			sklog.Errorf("Failed to look up commit %d: %s", commitNumber, err)
			continue
		}

		// It's possible that we don't have data for every single commit, so we
		// really need to know the range of commits that this regression
		// represents. So we go back to the previous sample we have in the trace
		// and find that commit. That is, the regression may have been created
		// on any commit in (previousCommitNumber, commitNumber] (inclusive of
		// commitNumber but exclusive of previousCommitNumber). This is way that
		// Gitiles and the Android build site work by default.
		previousCommitNumber := resp.Frame.DataFrame.Header[midPoint-1].Offset
		previousCommitDetails, err := c.perfGit.CommitFromCommitNumber(ctx, previousCommitNumber)
		if err != nil {
			sklog.Errorf("Failed to look up commit %d: %s", previousCommitNumber, err)
			continue
		}
		originalDataFrame := *resp.Frame.DataFrame
		for _, cl := range resp.Summary.Clusters {
			// Slim the DataFrame down to just the matching traces.
			df := dataframe.NewEmpty()
			df.Header = originalDataFrame.Header
			for _, key := range cl.Keys {
				df.TraceSet[key] = originalDataFrame.TraceSet[key]
			}
			df.BuildParamSet()
			resp.Frame.DataFrame = df

			// Update database if regression at the midpoint is found.
			if cl.StepPoint.Offset == commitNumber {
				if cl.StepFit.Status == stepfit.LOW && len(cl.Keys) >= cfg.MinimumNum && (cfg.DirectionAsString == alerts.DOWN || cfg.DirectionAsString == alerts.BOTH) {
					sklog.Infof("Found Low regression at %s. StepFit: %v Shortcut: %s AlertID: %s req: %#v", details.Subject, *cl.StepFit, cl.Shortcut, c.current.IDAsString, *req)
					c.updateStoreAndNotification(ctx, resp, cfg, commitNumber, cl, details, previousCommitDetails, key, true)
				}
				if cl.StepFit.Status == stepfit.HIGH && len(cl.Keys) >= cfg.MinimumNum && (cfg.DirectionAsString == alerts.UP || cfg.DirectionAsString == alerts.BOTH) {
					sklog.Infof("Found High regression at %s. StepFit: %v Shortcut: %s AlertID: %s req: %#v", details.Subject, *cl.StepFit, cl.Shortcut, c.current.IDAsString, *req)
					c.updateStoreAndNotification(ctx, resp, cfg, commitNumber, cl, details, previousCommitDetails, key, false)
				}
			}
		}
	}
}

func (c *Continuous) setCurrentConfig(cfg *alerts.Alert) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.current = cfg
}

// configsAndParamset is the type of channel that feeds Continuous.Run().
type configsAndParamSet struct {
	configs  []*alerts.Alert
	paramset paramtools.ReadOnlyParamSet
}

// configTracesMap provides a map of all the matching traces for a given
// alert config.
type configTracesMap map[alerts.Alert][]string

// getPubSubSubscription returns a pubsub.Subscription or an error if the
// subscription can't be established.
func (c *Continuous) getPubSubSubscription() (*pubsub.Subscription, error) {
	if c.instanceConfig.IngestionConfig.FileIngestionTopicName == "" {
		return nil, skerr.Fmt("Subscription name isn't set.")
	}

	ctx := context.Background()
	return sub.New(ctx, false, c.instanceConfig.IngestionConfig.SourceConfig.Project, c.instanceConfig.IngestionConfig.FileIngestionTopicName, maxParallelReceives)
}

func (c *Continuous) callProvider(ctx context.Context) ([]*alerts.Alert, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, config.QueryMaxRunTime)
	defer cancel()
	return c.provider.GetAllAlertConfigs(timeoutCtx, false)
}

func (c *Continuous) buildTraceConfigsMapChannelEventDriven(ctx context.Context) <-chan configTracesMap {
	ret := make(chan configTracesMap)
	sub, err := c.getPubSubSubscription()
	if err != nil {
		sklog.Errorf("Failed to create pubsub subscription, not doing event driven regression detection: %s", err)
	} else {

		// nackCounter is the number files we weren't able to ingest.
		nackCounter := metrics2.GetCounter("nack", nil)
		// ackCounter is the number files we were able to ingest.
		ackCounter := metrics2.GetCounter("ack", nil)
		go func() {
			for {
				if err := ctx.Err(); err != nil {
					sklog.Info("Channel context error %s", err)
					return
				}
				// Wait for PubSub events.
				err := sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
					sklog.Info("Received incoming Ingestion event.")
					// Set success to true if we should Ack the PubSub
					// message, otherwise the message will be Nack'd, and
					// PubSub will try to send the message again.
					success := false
					defer func() {
						if success {
							ackCounter.Inc(1)
							msg.Ack()
						} else {
							nackCounter.Inc(1)
							msg.Nack()
						}
					}()

					// Decode the event body.
					ie, err := ingestevents.DecodePubSubBody(msg.Data)
					if err != nil {
						sklog.Errorf("Failed to decode ingestion PubSub event: %s", err)
						// Data is malformed, ack it so we don't see it again.
						success = true
						return
					}

					sklog.Infof("IngestEvent received for : %q", ie.Filename)

					matchingConfigs, err := c.getTraceIdConfigsForIngestEvent(ctx, ie)
					if err != nil {
						sklog.Errorf("Failed retrieving relevant configs for incoming event.")
						success = false
						return
					}
					// If any configs match then emit the configsAndParamSet.
					if len(matchingConfigs) > 0 {
						sklog.Infof("Found %d matching configs for file %s", len(matchingConfigs), ie.Filename)
						ret <- matchingConfigs
					}
					success = true
				})
				if err != nil {
					sklog.Errorf("Failed receiving pubsub message: %s", err)
				}
			}
		}()
		sklog.Info("Started event driven clustering.")
	}

	return ret
}

func (c *Continuous) getTraceIdConfigsForIngestEvent(ctx context.Context, ie *ingestevents.IngestEvent) (configTracesMap, error) {
	// Filter all the configs down to just those that match
	// the incoming traces.
	configs, err := c.callProvider(ctx)
	if err != nil {
		sklog.Errorf("Failed to get list of configs: %s", err)
		return nil, err
	}

	return matchingConfigsFromTraceIDs(ie.TraceIDs, configs), nil
}

// buildConfigAndParamsetChannel returns a channel that will feed the configs
// and paramset that continuous regression detection should run over. In the
// future when Continuous.eventDriven is true this will be driven by PubSub
// events.
func (c *Continuous) buildConfigAndParamsetChannel(ctx context.Context) <-chan configsAndParamSet {
	ret := make(chan configsAndParamSet)
	go func() {
		for range time.Tick(c.pollingDelay) {
			if err := ctx.Err(); err != nil {
				sklog.Info("Channel context error %s", err)
				return
			}
			configs, err := c.callProvider(ctx)
			if err != nil {
				sklog.Errorf("Failed to get list of configs: %s", err)
				time.Sleep(time.Minute)
				continue
			}
			sklog.Infof("Found %d configs.", len(configs))
			// Shuffle the order of the configs.
			//
			// If we are running parallel continuous regression detectors then
			// shuffling means that we move through the configs in a different
			// order in each parallel Go routine and so find errors quicker,
			// otherwise we are just wasting cycles running the same exact
			// configs at the same exact time.
			rand.Shuffle(len(configs), func(i, j int) {
				configs[i], configs[j] = configs[j], configs[i]
			})

			sklog.Info("Configs shuffled")
			ret <- configsAndParamSet{
				configs:  configs,
				paramset: c.paramsProvider(),
			}
		}
	}()
	return ret
}

func (c *Continuous) updateStoreAndNotification(ctx context.Context, resp *regression.RegressionDetectionResponse, cfg *alerts.Alert, commitNumber types.CommitNumber,
	cl *clustering2.ClusterSummary, details provider.Commit, previousCommitDetails provider.Commit, key string, isLow bool) {
	ctx, span := trace.StartSpan(ctx, "regression.continuous.updateStoreAndNotification")
	defer span.End()

	updateNotification := true
	regression, err := c.store.GetRegression(ctx, commitNumber, key)
	if err != nil {
		sklog.Warningf("Regression not found or failed to retrieve! commitNumber=%s, key=%s, err=%s", commitNumber, key, err)
	}
	notificationID := getNotificationId(regression)
	if notificationID != "" {
		cl.NotificationID = notificationID
		// Do not update the notification if the regression direction is the same.
		if (isLow && regression.Low != nil) || (!isLow && regression.High != nil) {
			updateNotification = false
		}
	} else {
		var isNew bool
		var regressionID string
		if isLow {
			isNew, regressionID, err = c.store.SetLow(ctx, commitNumber, key, resp.Frame, cl)
		} else {
			isNew, regressionID, err = c.store.SetHigh(ctx, commitNumber, key, resp.Frame, cl)
		}
		if err != nil {
			sklog.Errorf("Failed to save newly found cluster: %s", err)
			return
		}
		sklog.Infof("Regression is detected! isLow:%s, regressionID: %s. IsNew: %s", isLow, regressionID, isNew)
		if isNew {
			c.regressionCounter.Inc(1)
			notificationID, err = c.notifier.RegressionFound(ctx, details, previousCommitDetails, cfg, cl, resp.Frame, regressionID)
			if err != nil {
				sklog.Errorf("Failed to send notification: %s", err)
			}
			cl.NotificationID = notificationID
			updateNotification = false
		}
	}
	if notificationID != "" {
		if isLow {
			_, _, err = c.store.SetLow(ctx, commitNumber, key, resp.Frame, cl)
		} else {
			_, _, err = c.store.SetHigh(ctx, commitNumber, key, resp.Frame, cl)
		}
		if err != nil {
			sklog.Errorf("Failed to save cluster with notification: %s", err)
		} else {
			sklog.Infof("Updated store! isLow:%s, NotificationID:%s, updateNotification:%b", isLow, notificationID, updateNotification)
		}
		if updateNotification {
			err = c.notifier.UpdateNotification(ctx, details, previousCommitDetails, cfg, cl, resp.Frame, notificationID)
			if err != nil {
				sklog.Errorf("Error updating notification with id %s: %v", notificationID, err)
			} else {
				sklog.Infof("Notification updated! NotificationID:%s", notificationID)
			}
		}
	}
}

// matchingConfigsFromTraceIDs returns a slice of Alerts that match at least one
// trace from the given traceIDs slice.
//
// Note that the Alerts returned may contain more restrictive Query values if
// the original Alert contains GroupBy parameters, while the original Alert
// remains unchanged.
func matchingConfigsFromTraceIDs(traceIDs []string, configs []*alerts.Alert) configTracesMap {
	matchingConfigs := map[alerts.Alert][]string{}
	if len(traceIDs) == 0 {
		return matchingConfigs
	}
	for _, config := range configs {
		q, err := query.NewFromString(config.Query)
		if err != nil {
			sklog.Errorf("An alert %q has an invalid query %q: %s", config.IDAsString, config.Query, err)
			continue
		}
		// If any traceID matches the query in the alert then it's an alert we should run.
		for _, key := range traceIDs {
			if q.Matches(key) {
				query, err := getConfigQueryForTrace(config, key)
				if err != nil {
					continue
				}

				configCopy := *config
				configCopy.Query = query
				_, ok := matchingConfigs[configCopy]
				if !ok {
					// Encountered this traceID for the first time.
					matchingConfigs[configCopy] = []string{}
				}
				matchingConfigs[configCopy] = append(matchingConfigs[configCopy], key)
			}
		}
	}
	return matchingConfigs
}

func getNotificationId(regression *regression.Regression) string {
	// The notification id is stored in the High/Low cluster summary of the regression object.
	if regression != nil && regression.High != nil && regression.High.NotificationID != "" {
		return regression.High.NotificationID
	}

	if regression != nil && regression.Low != nil && regression.Low.NotificationID != "" {
		return regression.Low.NotificationID
	}

	return ""
}

func getConfigQueryForTrace(config *alerts.Alert, traceID string) (string, error) {
	parsed, err := query.ParseKey(traceID)
	if err != nil {
		return "", err
	}
	query := config.Query
	if config.GroupBy != "" {
		// If we are in a GroupBy Alert then we should be able to
		// restrict the number of traces we look at by making the
		// Query more precise.
		for _, key := range config.GroupedBy() {
			if value, ok := parsed[key]; ok {
				query += fmt.Sprintf("&%s=%s", key, value)
			}
		}
	}

	return query, nil
}

// Run starts the continuous running of clustering over the last numCommits
// commits.
//
// Note that it never returns so it should be called as a Go routine.
func (c *Continuous) Run(ctx context.Context) {
	// TODO(jcgregorio) Add liveness metrics.
	sklog.Infof("Continuous starting.")

	if c.flags.EventDrivenRegressionDetection {
		c.RunEventDrivenClustering(ctx)
	} else {
		c.RunContinuousClustering(ctx)
	}
}

// RunEventDrivenClustering executes the regression detection based on events
// received from data ingestion.
func (c *Continuous) RunEventDrivenClustering(ctx context.Context) {
	// Range over a channel that returns a map containing the traceId as the key
	// and a list of matching alert configs as the value. These are processed
	// from the file that was just ingested and notification received over pubsub.
	for traceConfigMap := range c.buildTraceConfigsMapChannelEventDriven(ctx) {
		func(traceConfigMap configTracesMap) {
			ctx, span := trace.StartSpan(ctx, "regression.continuous.ProcessPubsubEvent")
			defer span.End()
			span.AddAttributes(trace.Int64Attribute("config_count", int64(len(traceConfigMap))))
			alertConfigs := make([]alerts.Alert, 0, len(traceConfigMap))
			for alertConfig := range traceConfigMap {
				alertConfigs = append(alertConfigs, alertConfig)
			}

			// At this point we have N alert configs matching the event, with T(n) matching traces per config.
			// We spawn $processAlertConfigsWorkerCount threads to process these in parallel (1 config per thread).
			// Each of these threads can spawn $processAlertConfigForTracesWorkerCount to gather trace data.
			err := util.ChunkIterParallelPool(ctx, len(alertConfigs), 1, processAlertConfigsWorkerCount, func(ctx context.Context, startIdx, endIdx int) error {
				config := alertConfigs[startIdx]
				if traces, ok := traceConfigMap[config]; ok {
					sklog.Infof("Clustering over %d traces for config %s", len(traces), config.IDAsString)
					// If the alert specifies StepFitGrouping (i.e Individual instead of KMeans)
					// we need to only query the paramset of the incoming data point instead of
					// the entire query in the alert.
					if config.Algo == types.StepFitGrouping {
						c.ProcessAlertConfigForTraces(ctx, config, traces)
					} else {
						c.ProcessAlertConfig(ctx, &config, doNotOverrideQuery)
					}
					sklog.Infof("Done with clustering over %d traces for config %s", len(traces), config.IDAsString)
					return nil
				} else {
					return skerr.Fmt("Alert config not found in traceConfigMap: %v", config)
				}
			})
			if err != nil {
				sklog.Errorf("Error processing alert configs: %v", err)
			}
		}(traceConfigMap)
	}
}

// ProcessAlertConfigForTrace runs the alert config on a specific trace id
func (c *Continuous) ProcessAlertConfigForTraces(ctx context.Context, alertConfig alerts.Alert, traceIds []string) {
	ctx, span := trace.StartSpan(ctx, "regression.continuous.ProcessAlertConfigForTraces")
	defer span.End()
	span.AddAttributes(trace.Int64Attribute("trace_count", int64(len(traceIds))))

	processAlertConfigForTracesChunkSize := 1
	if config.Config.Experiments.DfIterTraceSlicer {
		processAlertConfigForTracesChunkSize = 50
	}

	// Let's process the traces in parallel. Provide one trace per worker in parallel.
	// TODO(ashwinpv): It may be more deterministic to have the ability to query by
	// specific traceIds in dfbuilder instead of converting traceId to a query string.
	err := util.ChunkIterParallelPool(ctx, len(traceIds), processAlertConfigForTracesChunkSize, processAlertConfigForTracesWorkerCount, func(ctx context.Context, startIdx, endIdx int) error {
		if config.Config.Experiments.DfIterTraceSlicer {
			sklog.Infof("Trace Slicer enabled. Grouping traces into a single query.")
			paramset := paramtools.NewParamSet()
			// Group all traceIds into a single query for regression detection.
			for _, traceId := range traceIds[startIdx:endIdx] {
				paramset.AddParamsFromKey(traceId)
			}
			queryOverride := c.urlProvider.GetQueryStringFromParameters(paramset)
			c.ProcessAlertConfig(ctx, &alertConfig, queryOverride)
		} else {
			// Convert each traceId into a query for regression detection.
			for _, traceId := range traceIds[startIdx:endIdx] {
				sklog.Debugf("[AG] Processing trace id: %s", traceId)
				paramset := paramtools.NewParamSet()
				paramset.AddParamsFromKey(traceId)
				queryOverride := c.urlProvider.GetQueryStringFromParameters(paramset)
				c.ProcessAlertConfig(ctx, &alertConfig, queryOverride)
			}
		}

		return nil
	})
	if err != nil {
		sklog.Errorf("Error processing alert config for traces: %v", err)
	}
}

// RunContinuousClustering runs the regression detection on a continuous basis.
func (c *Continuous) RunContinuousClustering(ctx context.Context) {
	runsCounter := metrics2.GetCounter("perf_clustering_runs", nil)
	clusteringLatency := metrics2.NewTimer("perf_clustering_latency", nil)
	configsCounter := metrics2.GetCounter("perf_clustering_configs", nil)

	// Range over a channel here that supplies a slice of configs and a paramset
	// representing all the traceids we should be running over. The paramset is
	// the full paramset and the slice of configs is the full slice of configs in
	// the database.
	for cnp := range c.buildConfigAndParamsetChannel(ctx) {
		clusteringLatency.Start()
		for _, cfg := range cnp.configs {
			c.ProcessAlertConfig(ctx, cfg, doNotOverrideQuery)
			configsCounter.Inc(1)
		}
		clusteringLatency.Stop()
		runsCounter.Inc(1)
		configsCounter.Reset()
	}
}

// ProcessAlertConfig processes the supplied alert config to detect regressions
func (c *Continuous) ProcessAlertConfig(ctx context.Context, cfg *alerts.Alert, queryOverride string) {
	ctx, cancel := context.WithTimeout(ctx, timeoutForProcessAlertConfigPerTrace)
	defer cancel()

	ctx, span := trace.StartSpan(ctx, "regression.continuous.ProcessAlertConfig")
	defer span.End()

	c.setCurrentConfig(cfg)
	alertConfigLatencyTimer := metrics2.NewTimer(
		"perf_alertconfig_clustering_latency",
		map[string]string{
			"configName": cfg.DisplayName,
		})

	alertConfigLatencyTimer.Start()
	defer alertConfigLatencyTimer.Stop()
	// Smoketest the query, but only if we are not in event driven mode.
	if cfg.GroupBy != "" && !c.flags.EventDrivenRegressionDetection {
		sklog.Infof("Alert contains a GroupBy, doing a smoketest first: %q", cfg.DisplayName)
		u, err := url.ParseQuery(cfg.Query)
		if err != nil {
			sklog.Warningf("Alert failed smoketest: Alert contains invalid query: %q: %s", cfg.Query, err)
			return
		}
		q, err := query.New(u)
		if err != nil {
			sklog.Warningf("Alert failed smoketest: Alert contains invalid query: %q: %s", cfg.Query, err)
			return
		}

		var matches int64
		ctxutil.WithContextTimeout(ctx, config.QueryMaxRunTime, func(ctx context.Context) {
			matches, err = c.dfBuilder.NumMatches(ctx, q)
		})
		if err != nil {
			sklog.Warningf("Alert failed smoketest: %q Failed while trying generic query: %s", cfg.DisplayName, err)
			return
		}
		if matches == 0 {
			sklog.Warningf("Alert failed smoketest: %q Failed to get any traces for generic query.", cfg.DisplayName)
			return
		}
		sklog.Infof("Alert %q passed smoketest.", cfg.DisplayName)
	} else {
		sklog.Info("Not a GroupBy Alert.")
	}

	clusterResponseProcessor := func(ctx context.Context, req *regression.RegressionDetectionRequest, resps []*regression.RegressionDetectionResponse, message string) {
		c.reportRegressions(ctx, req, resps, cfg)
	}
	if cfg.Radius == 0 {
		cfg.Radius = c.flags.Radius
	}
	domain := types.Domain{
		N:   int32(c.flags.NumContinuous),
		End: time.Time{},
	}
	req := regression.NewRegressionDetectionRequest()
	req.Alert = cfg
	req.Domain = domain

	expandBaseRequest := regression.ExpandBaseAlertByGroupBy
	if c.flags.EventDrivenRegressionDetection {
		// Alert configs generated through
		// EventDrivenRegressionDetection are already given more precise
		// queries that take into account their GroupBy values and the
		// traces they matched.
		expandBaseRequest = regression.DoNotExpandBaseAlertByGroupBy
	}

	if queryOverride != doNotOverrideQuery {
		req.SetQuery(queryOverride)
	}

	var err error
	ctxutil.WithContextTimeout(ctx, config.QueryMaxRunTime, func(ctx context.Context) {
		err = regression.ProcessRegressions(ctx, req, clusterResponseProcessor, c.perfGit, c.shortcutStore, c.dfBuilder, c.paramsProvider(), expandBaseRequest, regression.ContinueOnError, c.instanceConfig.AnomalyConfig)
	})
	if err != nil {
		sklog.Warningf("Failed regression detection: Query: %q Error: %s", req.Query, err)
	}
}
