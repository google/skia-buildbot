package regression

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/dataframe"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/ingestevents"
	"go.skia.org/infra/perf/go/notify"
	"go.skia.org/infra/perf/go/shortcut"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/types"
)

const (
	// maxParallelReceives is the maximum number of Go routines used when
	// receiving PubSub messages.
	maxParallelReceives = 1

	// pollingClusteringDelay is the time to wait between clustering runs, but
	// only when not doing event driven regression detection.
	pollingClusteringDelay = 5 * time.Minute
)

// ConfigProvider is a function that's called to return a slice of
// alerts.Config. It is passed to NewContinuous.
type ConfigProvider func() ([]*alerts.Alert, error)

// ParamsetProvider is a function that's called to return the current paramset.
// It is passed to NewContinuous.
type ParamsetProvider func() paramtools.ParamSet

// ProgressCallback if a func that's called to return information on a currently running process.
type ProgressCallback func(message string)

// Current state of looking for regressions, i.e. the current commit and alert
// being worked on.
type Current struct {
	Commit  perfgit.Commit `json:"commit"`
	Alert   *alerts.Alert  `json:"alert"`
	Message string         `json:"message"`
}

// Continuous is used to run clustering on the last numCommits commits and
// look for regressions.
type Continuous struct {
	perfGit         *perfgit.Git
	store           Store
	shortcutStore   shortcut.Store
	numCommits      int // Number of recent commits to do clustering over.
	radius          int
	eventDriven     bool   // True if doing event driven regression detection.
	pubSubTopicName string // PubSub Topic name for incoming ingestion events.
	projectID       string // GCP Project name.
	local           bool   // Are we running locally or in prod.
	provider        ConfigProvider
	notifier        *notify.Notifier
	paramsProvider  ParamsetProvider
	dfBuilder       dataframe.DataFrameBuilder
	pollingDelay    time.Duration

	mutex   sync.Mutex // Protects current.
	current *Current
}

// NewContinuous creates a new *Continuous.
//
//   provider - Produces the slice of alerts.Config's that determine the clustering to perform.
//   numCommits - The number of commits to run the clustering over.
//   radius - The number of commits on each side of a commit to include when clustering.
func NewContinuous(
	perfGit *perfgit.Git,
	provider ConfigProvider,
	store Store,
	shortcutStore shortcut.Store,
	numCommits int,
	radius int,
	notifier *notify.Notifier,
	paramsProvider ParamsetProvider,
	dfBuilder dataframe.DataFrameBuilder,
	local bool,
	projectID string,
	fileIngestionTopicName string,
	eventDriven bool) *Continuous {
	return &Continuous{
		perfGit:         perfGit,
		store:           store,
		shortcutStore:   shortcutStore,
		numCommits:      numCommits,
		radius:          radius,
		provider:        provider,
		eventDriven:     eventDriven,
		pubSubTopicName: fileIngestionTopicName,
		local:           local,
		projectID:       projectID,
		notifier:        notifier,
		current:         &Current{},
		paramsProvider:  paramsProvider,
		dfBuilder:       dfBuilder,
		pollingDelay:    pollingClusteringDelay,
	}
}

// CurrentStatus returns the current status of regression detection.
func (c *Continuous) CurrentStatus() Current {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return *c.current
}

func (c *Continuous) reportRegressions(ctx context.Context, req *RegressionDetectionRequest, resps []*RegressionDetectionResponse, cfg *alerts.Alert) {
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
		for _, cl := range resp.Summary.Clusters {
			// Zero out the DataFrame ParamSet since it is never used.
			resp.Frame.DataFrame.ParamSet = paramtools.ParamSet{}
			// Update database if regression at the midpoint is found.
			if cl.StepPoint.Offset == commitNumber {
				if cl.StepFit.Status == stepfit.LOW && len(cl.Keys) >= cfg.MinimumNum && (cfg.DirectionAsString == alerts.DOWN || cfg.DirectionAsString == alerts.BOTH) {
					sklog.Infof("Found Low regression at %s: StepFit: %v Shortcut: %s AlertID: %d %d req: %#v", details.Subject, *cl.StepFit, cl.Shortcut, cfg.ID, c.current.Alert.ID, *req)
					isNew, err := c.store.SetLow(ctx, commitNumber, key, resp.Frame, cl)
					if err != nil {
						sklog.Errorf("Failed to save newly found cluster: %s", err)
						continue
					}
					if isNew {
						if err := c.notifier.Send(details, cfg, cl); err != nil {
							sklog.Errorf("Failed to send notification: %s", err)
						}
					}
				}
				if cl.StepFit.Status == stepfit.HIGH && len(cl.Keys) >= cfg.MinimumNum && (cfg.DirectionAsString == alerts.UP || cfg.DirectionAsString == alerts.BOTH) {
					sklog.Infof("Found High regression at %s: StepFit: %v Shortcut: %s AlertID: %d %d req: %#v", details.Subject, *cl.StepFit, cl.Shortcut, cfg.ID, c.current.Alert.ID, *req)
					isNew, err := c.store.SetHigh(ctx, commitNumber, key, resp.Frame, cl)
					if err != nil {
						sklog.Errorf("Failed to save newly found cluster for alert %q length=%d: %s", key, len(cl.Keys), err)
						continue
					}
					if isNew {
						if err := c.notifier.Send(details, cfg, cl); err != nil {
							sklog.Errorf("Failed to send notification: %s", err)
						}
					}
				}
			}
		}
	}
}

func (c *Continuous) setCurrentConfig(cfg *alerts.Alert) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.current.Alert = cfg
}

func (c *Continuous) progressCallback(message string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.current.Message = message
}

// configsAndParamset is the type of channel that feeds Continuous.Run().
type configsAndParamSet struct {
	configs  []*alerts.Alert
	paramset paramtools.ParamSet
}

// getPubSubSubscription returns a pubsub.Subscription or an error if the
// subscription can't be established.
func (c *Continuous) getPubSubSubscription() (*pubsub.Subscription, error) {
	if c.pubSubTopicName == "" {
		return nil, skerr.Fmt("Subscription name isn't set.")
	}
	hostname, err := os.Hostname()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, c.projectID)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// When running in production we have every instance use the same topic name
	// so that they load-balance pulling items from the topic.
	topicName := fmt.Sprintf("%s-%s", c.pubSubTopicName, "prod")
	if c.local {
		// When running locally create a new topic for every host.
		topicName = fmt.Sprintf("%s-%s", c.pubSubTopicName, hostname)
	}
	sub := client.Subscription(topicName)
	ok, err := sub.Exists(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed checking subscription existence")
	}
	if !ok {
		sub, err = client.CreateSubscription(ctx, topicName, pubsub.SubscriptionConfig{
			Topic: client.Topic(c.pubSubTopicName),
		})
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed creating subscription")
		}
	}

	// How many Go routines should be processing messages?
	sub.ReceiveSettings.MaxOutstandingMessages = maxParallelReceives
	sub.ReceiveSettings.NumGoroutines = maxParallelReceives

	return sub, nil
}

// buildConfigAndParamsetChannel returns a channel that will feed the configs
// and paramset that continuous regression detection should run over. In the
// future when Continuous.eventDriven is true this will be driven by PubSub
// events.
func (c *Continuous) buildConfigAndParamsetChannel() <-chan configsAndParamSet {
	ret := make(chan configsAndParamSet)

	if c.eventDriven {
		sub, err := c.getPubSubSubscription()
		if err != nil {
			sklog.Errorf("Failed to create pubsub subscription, not doing event driven regression detection: %s", err)
			// Just fall through and look for regressions over all the Alerts continuously.
		} else {

			// nackCounter is the number files we weren't able to ingest.
			nackCounter := metrics2.GetCounter("nack", nil)
			// ackCounter is the number files we were able to ingest.
			ackCounter := metrics2.GetCounter("ack", nil)
			go func() {
				for {
					// Wait for PubSub events.
					err := sub.Receive(context.Background(), func(ctx context.Context, msg *pubsub.Message) {
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
						// Filter all the configs down to just those that match
						// the incoming traces.
						configs, err := c.provider()
						if err != nil {
							sklog.Errorf("Failed to get list of configs: %s", err)
							// An error not related to the event, nack so we try again later.
							success = false
							return
						}
						matchingConfigs := []*alerts.Alert{}
						for _, config := range configs {
							q, err := query.NewFromString(config.Query)
							if err != nil {
								sklog.Errorf("An alert %q has an invalid query %q: %s", config.ID, config.Query, err)
								continue
							}
							// If any traceID matches the query in the alert then it's an alert we should run.
							for _, key := range ie.TraceIDs {
								if q.Matches(key) {
									matchingConfigs = append(matchingConfigs, config)
									break
								}
							}
						}

						// If any configs match then emit the configsAndParamSet.
						if len(matchingConfigs) > 0 {
							ret <- configsAndParamSet{
								configs:  matchingConfigs,
								paramset: ie.ParamSet,
							}
						}
						success = true
					})
					if err != nil {
						sklog.Errorf("Failed receiving pubsub message: %s", err)
					}
				}
			}()
			sklog.Info("Started event driven clustering.")
			return ret
		}
	} else {
		sklog.Info("Not event driven clustering.")
	}
	go func() {
		for range time.Tick(c.pollingDelay) {
			configs, err := c.provider()
			if err != nil {
				sklog.Errorf("Failed to get list of configs: %s", err)
				time.Sleep(time.Minute)
				continue
			}
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

			ret <- configsAndParamSet{
				configs:  configs,
				paramset: c.paramsProvider(),
			}
		}
	}()
	return ret
}

// Run starts the continuous running of clustering over the last numCommits
// commits.
//
// Note that it never returns so it should be called as a Go routine.
func (c *Continuous) Run(ctx context.Context) {
	runsCounter := metrics2.GetCounter("perf_clustering_runs", nil)
	clusteringLatency := metrics2.NewTimer("perf_clustering_latency", nil)
	configsCounter := metrics2.GetCounter("perf_clustering_configs", nil)

	// TODO(jcgregorio) Add liveness metrics.
	sklog.Infof("Continuous starting.")

	// Instead of ranging over time, we should be ranging over PubSub events
	// that list the ids of the last file that was ingested. Then we should loop
	// over each config and see if that list of trace ids matches any configs,
	// and if so at that point we start running the regresions. But we also want
	// to preserve continuous regression detection for the cases where it makes
	// sense, e.g. Skia.
	//
	// So we can actually range over a channel here that supplies a slice of
	// configs and a paramset representing all the traceids we should be running
	// over. If this is just a timer then the paramset is the full paramset and
	// the slice of configs is just the full slice of configs. If it is PubSub
	// driven then the paramset is built from the list of trace ids we received
	// and the list of configs is built by matching the full list of configs
	// against the list of incoming trace ids.
	//
	for cnp := range c.buildConfigAndParamsetChannel() {
		clusteringLatency.Start()
		sklog.Infof("Clustering over %d configs.", len(cnp.configs))
		for _, cfg := range cnp.configs {
			c.setCurrentConfig(cfg)

			// Smoketest the query, but only if we are not in event driven mode.
			if cfg.GroupBy != "" && !c.eventDriven {
				sklog.Infof("Alert contains a GroupBy, doing a smoketest first: %q", cfg.DisplayName)
				u, err := url.ParseQuery(cfg.Query)
				if err != nil {
					sklog.Warningf("Alert failed smoketest: Alert contains invalid query: %q: %s", cfg.Query, err)
					continue
				}
				q, err := query.New(u)
				if err != nil {
					sklog.Warningf("Alert failed smoketest: Alert contains invalid query: %q: %s", cfg.Query, err)
					continue
				}
				// Should be changed to PreflightQuery.
				df, err := c.dfBuilder.NewNFromQuery(context.Background(), time.Time{}, q, 20, nil)
				if err != nil {
					sklog.Warningf("Alert failed smoketest: %q Failed while trying generic query: %s", cfg.DisplayName, err)
					continue
				}
				if len(df.TraceSet) == 0 {
					sklog.Warningf("Alert failed smoketest: %q Failed to get any traces for generic query.", cfg.DisplayName)
					continue
				}
				sklog.Infof("Alert %q passed smoketest.", cfg.DisplayName)
			}

			clusterResponseProcessor := func(req *RegressionDetectionRequest, resps []*RegressionDetectionResponse, message string) {
				c.reportRegressions(ctx, req, resps, cfg)
			}
			if cfg.Radius == 0 {
				cfg.Radius = c.radius
			}
			domain := types.Domain{
				N:   int32(c.numCommits),
				End: time.Time{},
			}
			RegressionsForAlert(ctx, cfg, domain, cnp.paramset, c.shortcutStore, clusterResponseProcessor, c.perfGit, c.dfBuilder, c.progressCallback)
			configsCounter.Inc(1)
		}
		clusteringLatency.Stop()
		runsCounter.Inc(1)
		configsCounter.Reset()
	}
}
