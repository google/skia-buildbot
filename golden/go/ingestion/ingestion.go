package ingestion

import (
	"context"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/eventbus"
)

const (
	// nConcurrentProcessors is the maximum number of go-routines that run Processors.
	// The number is chosen experimentally and should be adjusted to optimize throughput.
	// It can be small, as the number of ingesters can be increased with more
	// kubernetes replicas.
	nConcurrentProcessors = 40

	// eventChanSize is the buffer size of the events channel. Most of the time
	// that channel should be almost empty, but this ensures we buffer events if
	// processing input files take longer or there is a large number of concurrent events.
	eventChanSize = 500
)

// Ingester is the main type that drives ingestion for a single type.
type Ingester struct {
	id             string
	vcs            vcsinfo.VCS
	nCommits       int
	minDuration    time.Duration
	runEvery       time.Duration
	sources        []Source
	processor      Processor
	ingestionStore IngestionStore
	eventBus       eventbus.EventBus

	// eventProcessMetrics contains all events we are interested in.
	eventProcessMetrics *processMetrics
}

// newIngester creates a new Ingester with the given id and configuration around
// the supplied vcs (version control system), input sources and Processor instance.
// The Ingester is event driven by storage events with a background process that polls
// the storage locations. The given eventBus cannot be nil and must be shared with the sources
// that are passed. To only do polling-based ingestion use an in-memory eventbus
// (created via eventbus.New()). To drive ingestion from storage events use a PubSub-based
// eventbus (created via the gevent.New(...) function).
//
func newIngester(ingesterID string, ingesterConf Config, vcs vcsinfo.VCS, sources []Source, processor Processor, ingestionStore IngestionStore, eventBus eventbus.EventBus) (*Ingester, error) {
	if eventBus == nil || ingestionStore == nil {
		return nil, skerr.Fmt("eventBus and ingestionStore cannot be nil")
	}

	minDuration := time.Duration(ingesterConf.MinDays) * time.Hour * 24
	minDuration += time.Duration(ingesterConf.MinHours) * time.Hour

	ret := &Ingester{
		id:                  ingesterID,
		vcs:                 vcs,
		nCommits:            ingesterConf.NCommits,
		minDuration:         minDuration,
		runEvery:            ingesterConf.RunEvery.Duration,
		sources:             sources,
		processor:           processor,
		ingestionStore:      ingestionStore,
		eventBus:            eventBus,
		eventProcessMetrics: newProcessMetrics(ingesterID),
	}
	return ret, nil
}

// Start starts the Ingester in a new goroutine.
func (i *Ingester) Start(ctx context.Context) error {
	if i.processor == nil {
		return skerr.Fmt("processor cannot be nil")
	}
	if len(i.sources) == 0 {
		return skerr.Fmt("at least one source must have been provided")
	}

	concurrentProc := make(chan bool, nConcurrentProcessors)
	resultChan, err := i.getInputChannel(ctx)
	if err != nil {
		return skerr.Wrapf(err, "retrieving input channel")
	}

	// Continuously catch events from all input sources and push the data to the processor.
	go func() {
		var resultFile ResultFileLocation = nil
		for {
			select {
			case resultFile = <-resultChan:
			case <-ctx.Done():
				return
			}

			// get a slot in line to call Process
			concurrentProc <- true
			go func(resultFile ResultFileLocation) {
				defer func() { <-concurrentProc }()
				i.processResult(ctx, resultFile)
			}(resultFile)
		}
	}()
	return nil
}

// Close stops the ingestion process. Currently only used for testing. It's mainly intended
// to terminate as many goroutines as possible.
func (i *Ingester) Close() error {
	// Close the liveness metrics.
	i.eventProcessMetrics.pollingLiveness.Close()
	i.eventProcessMetrics.processLiveness.Close()
	return nil
}

func (i *Ingester) getInputChannel(ctx context.Context) (<-chan ResultFileLocation, error) {
	eventChan := make(chan ResultFileLocation, eventChanSize)

	for _, source := range i.sources {
		if err := source.SetEventChannel(eventChan); err != nil {
			return nil, skerr.Wrapf(err, "setting event channel for source %v", source)
		}

		// Watch the source and feed anything not found in the IngestionStore
		go i.watchSource(ctx, source)
	}
	return eventChan, nil
}

// watchSource starts a background process that poll the given source in
// scheduled intervals (controlled by i.runEvery) and generates synthetic
// storage events if the files in the source have not been ingested yet.
func (i *Ingester) watchSource(ctx context.Context, source Source) {
	if i.minDuration == 0 {
		sklog.Infof("Not going to do polling because minDuration == 0")
		return
	}
	sklog.Infof("Watching source %s", source.ID())

	// RepeatCtx will run the function right away and then in intervals of 'runEvery'.
	util.RepeatCtx(ctx, i.runEvery, func(ctx context.Context) {
		// Get the start of the time range that we are polling.
		startTime, err := i.getStartTimeOfInterest(ctx, time.Now())
		if err != nil {
			sklog.Errorf("Unable to get commit range of interest: %s", err)
			return
		}

		rfCh := source.Poll(startTime.Unix(), time.Now().Unix())
		processed := int64(0)
		ignored := int64(0)
		sklog.Infof("Polling starting at %s [UTC]", startTime)
		for rf := range rfCh {
			// It is a rare case that the pubsub event got lost, so we check to see
			// if we already processed the file before re-queuing it.
			if i.inProcessedFiles(ctx, rf.Name(), rf.MD5()) {
				ignored++
				continue
			}
			processed++

			bucketID, objectID := rf.StorageIDs()
			i.eventBus.PublishStorageEvent(eventbus.NewStorageEvent(bucketID, objectID, rf.TimeStamp(), rf.MD5()))
		}
		i.eventProcessMetrics.ignoredByPollingGauge.Update(ignored)
		i.eventProcessMetrics.processedByPollingGauge.Update(processed)
		i.eventProcessMetrics.pollingLiveness.Reset()
		sklog.Infof("Watcher for %s received/processed/ignored: %d/%d/%d", source.ID(), ignored+processed, processed, ignored)
	})
}

// inProcessedFiles returns true if the given md5 hash is in the list of
// already processed files.
func (i *Ingester) inProcessedFiles(ctx context.Context, name, md5 string) bool {
	ret, err := i.ingestionStore.WasIngested(ctx, name, md5)
	if err != nil {
		sklog.Errorf("Error checking ingestionstore for %s %s: %s", name, md5, err)
		return false
	}
	return ret
}

// addToProcessedFiles adds the given list of md5 hashes to the list of
// file that have been already processed.
func (i *Ingester) addToProcessedFiles(ctx context.Context, name, md5 string, ts time.Time) {
	if err := i.ingestionStore.SetIngested(ctx, name, md5, ts); err != nil {
		sklog.Errorf("Error setting %s %s in ingestionstore: %s", name, md5, err)
	}
}

// processResult processes a single result file.
func (i *Ingester) processResult(ctx context.Context, rfl ResultFileLocation) {
	// processResult does not check the inProcessedFiles because we want to retain the ability
	// to force a re-process via bt_reingester or other means.
	name, md5 := rfl.Name(), rfl.MD5()
	err := i.processor.Process(ctx, rfl)
	if err != nil {
		if err != IgnoreResultsFileErr {
			sklog.Errorf("Failed to ingest %s: %s", name, err)
		}
		return
	}
	i.addToProcessedFiles(ctx, name, md5, time.Now())
	i.eventProcessMetrics.processLiveness.Reset()
}

// getStartTimeOfInterest returns the start time of input files we are interested in.
// We will then poll for input files from startTime to now. This is computed using the
// configured NCommits and MinDays for the Ingester.
func (i *Ingester) getStartTimeOfInterest(ctx context.Context, now time.Time) (time.Time, error) {
	// If there is no vcs, use the minDuration field of the Ingester to calculate
	// the start time. If nCommits is 0 (e.g. TryJobs), then don't bother with the VCS - just
	// return the time delta.
	if i.vcs == nil || i.nCommits == 0 {
		return now.Add(-i.minDuration), nil
	}

	// Make sure the VCS is up to date.
	if err := i.vcs.Update(ctx, true, false); err != nil {
		return time.Time{}, skerr.Wrap(err)
	}

	// Get the desired number of commits in the desired time frame.
	delta := -i.minDuration
	hashes := i.vcs.From(now.Add(delta))

	// If the number of required commits is not covered by this time
	// frame then keep adding more (up until we are scanning the last year of data, at which point
	// something must be wrong or the repository is just very new).
	if len(hashes) < i.nCommits {
		delta *= 2
		for ; len(hashes) < i.nCommits && delta > -365*24*time.Hour; delta *= 2 {
			hashes = i.vcs.From(now.Add(delta))
		}

		// In case we have retrieved too many commits.
		if len(hashes) > i.nCommits {
			hashes = hashes[len(hashes)-i.nCommits:]
		}
	}

	if len(hashes) == 0 {
		return time.Time{}, skerr.Fmt("no commits found in last year")
	}

	// Get the commit time of the first commit of interest.
	detail, err := i.vcs.Details(ctx, hashes[0], false)
	if err != nil {
		return time.Time{}, skerr.Wrap(err)
	}

	return detail.Timestamp, nil
}

// processMetrics contains the metrics we are interested for processing results.
type processMetrics struct {
	ignoredByPollingGauge   metrics2.Int64Metric
	processedByPollingGauge metrics2.Int64Metric
	pollingLiveness         metrics2.Liveness
	processLiveness         metrics2.Liveness
}

const (
	ingestionMetric    = "ingestion"
	ingestionMetricTag = "metric"
	idTag              = "ingester"
	sourceTag          = "source"
)

// newProcessMetrics instantiates the metrics to track processing and registers them
// with the metrics package.
func newProcessMetrics(id string) *processMetrics {
	return &processMetrics{
		ignoredByPollingGauge: metrics2.GetInt64Metric(ingestionMetric, map[string]string{
			idTag:              id,
			sourceTag:          "poll",
			ingestionMetricTag: "ignored",
		}),
		processedByPollingGauge: metrics2.GetInt64Metric(ingestionMetric, map[string]string{
			idTag:              id,
			sourceTag:          "poll",
			ingestionMetricTag: "processed",
		}),
		pollingLiveness: metrics2.NewLiveness(id, map[string]string{
			sourceTag:          "poll",
			ingestionMetricTag: "since-last-run",
		}),
		processLiveness: metrics2.NewLiveness(id, map[string]string{
			sourceTag:          "gcs_event",
			ingestionMetricTag: "last-successful-process",
		}),
	}
}
