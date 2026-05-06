package backfill

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"time"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/pubsub/sub"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/dfiter"
	"go.skia.org/infra/perf/go/progress"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/types"
)

// AlertProcessor defines the interface for processing alert configs.
// This breaks the circular dependency between continuous and backfill packages.
type AlertProcessor interface {
	ProcessAlertConfig(ctx context.Context, cfg *alerts.Alert, queryOverride string, dfProvider *dfiter.DfProvider, domain *types.Domain, skipNotifications bool) error
	ProcessAlertConfigForTraces(ctx context.Context, alertConfig alerts.Alert, traceIds []string, dfProvider *dfiter.DfProvider, domain *types.Domain, skipNotifications bool, failFast bool) error
}

// Listener listens for backfill requests on a PubSub topic.
type Listener struct {
	instanceConfig *config.InstanceConfig
	provider       alerts.ConfigProvider
	dfBuilder      dataframe.DataFrameBuilder
	processor      AlertProcessor
	numContinuous  int
}

// NewListener creates a new Listener.
func NewListener(cfg *config.InstanceConfig, provider alerts.ConfigProvider, dfBuilder dataframe.DataFrameBuilder, processor AlertProcessor, numContinuous int) *Listener {
	return &Listener{
		instanceConfig: cfg,
		provider:       provider,
		dfBuilder:      dfBuilder,
		processor:      processor,
		numContinuous:  numContinuous,
	}
}

// Run starts listening for backfill requests.
func (l *Listener) Run(ctx context.Context) {
	if l.instanceConfig.AnomalyConfig.BackfillTopicName == "" {
		sklog.Info("BackfillTopicName is not set, not running backfill listener.")
		return
	}

	concurrency := l.instanceConfig.AnomalyConfig.BackfillConcurrency
	if concurrency <= 0 {
		concurrency = 1
	}
	sub, err := sub.New(ctx, false, l.instanceConfig.IngestionConfig.SourceConfig.Project, l.instanceConfig.AnomalyConfig.BackfillTopicName, concurrency)
	if err != nil {
		sklog.Errorf("Failed to create pubsub subscription for backfill: %s", err)
		return
	}

	sklog.Infof("Listening for backfill requests on topic: %s", l.instanceConfig.AnomalyConfig.BackfillTopicName)

	err = sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		// timeout based on ack_deadline_seconds for subscriptions we read messages
		timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()

		alertID, shouldAck, err := l.processBackfillMessage(timeoutCtx, msg)
		alertIDStr := ""
		if alertID != 0 {
			alertIDStr = strconv.FormatInt(alertID, 10)
		}
		tags := map[string]string{"alertid": alertIDStr}

		runsTotal := metrics2.GetCounter("perf_backfill_runs_total", tags)
		failureTotal := metrics2.GetCounter("perf_backfill_failure_total", tags)
		errorsTotal := metrics2.GetCounter("perf_backfill_errors_total", tags)
		successTotal := metrics2.GetCounter("perf_backfill_success_total", tags)

		if alertID != 0 {
			runsTotal.Inc(1)
		}

		if err != nil {
			sklog.Errorf("Failed to process backfill message %s (Data: %s): %s", msg.ID, string(msg.Data), err)
			if alertID != 0 {
				failureTotal.Inc(1)
				errCount := int64(regression.CountErrors([]error{err}))
				errorsTotal.Inc(errCount)
			}
			if shouldAck {
				sklog.Warningf("Acking unprocessable message %s to avoid infinite retry.", msg.ID)
				msg.Ack()
			} else {
				msg.Nack()
			}
		} else {
			if alertID != 0 {
				successTotal.Inc(1)
			}
			msg.Ack()
		}
	})
	if err != nil {
		sklog.Errorf("PubSub receive failed: %s", err)
	}
}

func (l *Listener) processBackfillMessage(ctx context.Context, msg *pubsub.Message) (int64, bool, error) {
	var req regression.BackfillRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		return 0, true, skerr.Wrapf(err, "Failed to decode backfill request")
	}

	sklog.Infof("Received backfill request for alert: %d, RequestID: %s, Request: %s", req.AlertID, req.RequestID, string(msg.Data))

	shouldAck, err := l.processBackfillRequest(ctx, &req)
	if err != nil {
		return req.AlertID, shouldAck, skerr.Wrapf(err, "Failed to process backfill request")
	}

	return req.AlertID, true, nil
}

func (l *Listener) processBackfillRequest(ctx context.Context, req *regression.BackfillRequest) (bool, error) {
	id := req.AlertID
	cfg, err := l.provider.GetAlertConfig(id)
	if err != nil {
		return true, skerr.Wrapf(err, "Failed to get alert config for ID %d", id)
	}

	prog := progress.New()
	sklog.Infof("Processing alert backfill for: %s", cfg.DisplayName)

	if err := l.processSingleAlertBackfill(ctx, cfg, req, prog); err != nil {
		return false, skerr.Wrapf(err, "Failed to process backfill for alert %s", cfg.DisplayName)
	}

	return true, nil
}

func (l *Listener) processSingleAlertBackfill(ctx context.Context, cfg *alerts.Alert, req *regression.BackfillRequest, prog progress.Progress) error {
	domain := types.Domain{
		N:   int32(l.numContinuous),
		End: time.Unix(req.End, 0),
	}

	u, err := url.ParseQuery(cfg.Query)
	if err != nil {
		return skerr.Wrapf(err, "Failed to parse query for alert")
	}
	q, err := query.New(u)
	if err != nil {
		return skerr.Wrapf(err, "Failed to create query for alert")
	}

	if len(req.TraceIDs) > 0 {
		var filteredTraceIDs []string
		for _, traceID := range req.TraceIDs {
			if q.Matches(traceID) {
				filteredTraceIDs = append(filteredTraceIDs, traceID)
			} else {
				sklog.Infof("Skipping trace %s as it does not match alert query %q for Alert ID: %d", traceID, cfg.Query, req.AlertID)
			}
		}
		if len(filteredTraceIDs) == 0 {
			sklog.Infof("No provided traces matched the alert query %q for Alert ID: %d. Skipping backfill.", cfg.Query, req.AlertID)
			return nil
		}
		return l.processor.ProcessAlertConfigForTraces(ctx, *cfg, filteredTraceIDs, nil, &domain, !req.SendNotifications, true)
	}

	if req.LoadAllTracesTogether {
		return l.processor.ProcessAlertConfig(ctx, cfg, "", nil, &domain, !req.SendNotifications)
	}

	df, err := l.dfBuilder.NewNFromQuery(ctx, domain.End, q, domain.N, prog)
	if err != nil {
		return skerr.Wrapf(err, "Failed to load traces for alert")
	}

	keys := make([]string, 0, len(df.TraceSet))
	for key := range df.TraceSet {
		keys = append(keys, key)
	}

	return l.processor.ProcessAlertConfigForTraces(ctx, *cfg, keys, nil, &domain, !req.SendNotifications, true)
}
