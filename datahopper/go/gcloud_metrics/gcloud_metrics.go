package gcloud_metrics

import (
	"context"
	"fmt"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3"
	"github.com/googleapis/gax-go/v2"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// metricsPeriod was chosen to be large enough to contain at least one data
	// point, with some extra buffer, since Cloud Monitoring has a sample rate
	// of one data point per minute.
	metricsPeriod      = 3 * time.Minute
	metricsInterval    = time.Minute
	pubsubMetricPrefix = "pubsub.googleapis.com/subscription/"
)

var (
	pubsubMetrics = map[string]string{
		pubsubMetricPrefix + "num_undelivered_messages":   "pubsub_num_undelivered_messages",
		pubsubMetricPrefix + "oldest_unacked_message_age": "pubsub_oldest_unacked_message_age_s",
	}
	pubsubIncludeLabels = []string{"subscription_id"}
)

// StartGCloudMetrics begins ingesting metrics from the GCloud Monitoring API.
func StartGCloudMetrics(ctx context.Context, projects []string, ts oauth2.TokenSource) error {
	metricsClientInner, err := monitoring.NewMetricClient(ctx, option.WithTokenSource(ts))
	if err != nil {
		return skerr.Wrap(err)
	}
	metricsClient := &metricClientWrapper{
		metricClient: metricsClientInner,
	}
	for _, project := range projects {
		project := project // Prevents re-use of the variable causing problems in the goroutine.
		lvReportGCloudMetrics := metrics2.NewLiveness("last_successful_report_gcloud_metrics", map[string]string{
			"project": project,
		})
		oldMetrics := map[metrics2.Float64Metric]struct{}{}
		go util.RepeatCtx(ctx, metricsInterval, func(ctx context.Context) {
			newMetrics, err := ingestMetricsForProject(ctx, metricsClient, project)
			if err != nil {
				sklog.Error(err)
				return
			}
			newMetricsMap := make(map[metrics2.Float64Metric]struct{}, len(newMetrics))
			for _, m := range newMetrics {
				newMetricsMap[m] = struct{}{}
			}
			var cleanup []metrics2.Float64Metric
			for m := range oldMetrics {
				if _, ok := newMetricsMap[m]; !ok {
					cleanup = append(cleanup, m)
				}
			}
			if len(cleanup) > 0 {
				failedDelete := cleanupOldMetrics(cleanup)
				for _, m := range failedDelete {
					newMetricsMap[m] = struct{}{}
				}
			}
			oldMetrics = newMetricsMap
			lvReportGCloudMetrics.Reset()
		})
	}
	return nil
}

// cleanupOldMetrics deletes old metrics. This fixes the case where metrics no
// longer appear upstream but their metrics hang around without being updated.
func cleanupOldMetrics(oldMetrics []metrics2.Float64Metric) []metrics2.Float64Metric {
	failedDelete := []metrics2.Float64Metric{}
	for _, m := range oldMetrics {
		if err := m.Delete(); err != nil {
			sklog.Warningf("Failed to delete metric: %s", err)
			failedDelete = append(failedDelete, m)
		}
	}
	return failedDelete
}

// ingestMetricsForProject ingests all GCloud metrics for a particular projects.
func ingestMetricsForProject(ctx context.Context, metricsClient MetricClient, project string) ([]metrics2.Float64Metric, error) {
	rv := []metrics2.Float64Metric{}

	// PubSub metrics.
	pubsubMetrics, err := ingestPubSubMetrics(ctx, metricsClient, project)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	rv = append(rv, pubsubMetrics...)

	// TODO: Additional types of metrics go here, following the same pattern as
	// PubSub above.

	return rv, nil
}

// ingestPubSubMetrics ingests Cloud PubSub metrics for a project.
func ingestPubSubMetrics(ctx context.Context, metricsClient MetricClient, project string) ([]metrics2.Float64Metric, error) {
	endTs := now.Now(ctx)
	endTime := timestamppb.New(endTs)
	startTime := timestamppb.New(endTs.Add(-metricsPeriod))
	rv := []metrics2.Float64Metric{}
	for metricType, measurement := range pubsubMetrics {
		metrics, err := ingestTimeSeries(ctx, metricsClient, project, metricType, measurement, pubsubIncludeLabels, startTime, endTime)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		rv = append(rv, metrics...)
	}
	return rv, nil
}

// MetricClient provides an abstraction for monitoring.MetricClient, used for
// testing.
type MetricClient interface {
	ListTimeSeries(context.Context, *monitoringpb.ListTimeSeriesRequest, ...gax.CallOption) TimeSeriesIterator
}

// TimeSeriesIterator provides an abstraction for monitoring.TimeSeriesIterator,
// used for testing.
type TimeSeriesIterator interface {
	Next() (*monitoringpb.TimeSeries, error)
}

// metricClientWrapper wraps a monitoring.MetricClient to implement
// MetricClient.
type metricClientWrapper struct {
	metricClient *monitoring.MetricClient
}

// ListTimeSeries implements MetricClient.
func (w *metricClientWrapper) ListTimeSeries(ctx context.Context, req *monitoringpb.ListTimeSeriesRequest, opts ...gax.CallOption) TimeSeriesIterator {
	return w.metricClient.ListTimeSeries(ctx, req, opts...)
}

// ingestTimeSeries ingests data from one or more time series matching the given
// metricType within the given project. The data is stored under the given
// measurement name. The includeLabels must deduplicate the time series
// sufficiently to prevent writing conflicting values to the same metric.
func ingestTimeSeries(ctx context.Context, metricsClient MetricClient, project, metricType, measurement string, includeLabels []string, startTime, endTime *timestamppb.Timestamp) ([]metrics2.Float64Metric, error) {
	rv := []metrics2.Float64Metric{}
	req := &monitoringpb.ListTimeSeriesRequest{
		Name:   fmt.Sprintf("projects/%s", project),
		Filter: fmt.Sprintf("metric.type = %q", metricType),
		Interval: &monitoringpb.TimeInterval{
			EndTime:   endTime,
			StartTime: startTime,
		},
		// ListTimeSeriesRequest_FULL gets us the actual data points from the
		// time series, as opposed to just the metadata.
		View: monitoringpb.ListTimeSeriesRequest_FULL,
	}
	iter := metricsClient.ListTimeSeries(ctx, req)
	for {
		timeSeries, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		if len(timeSeries.Points) > 0 {
			// Just use the very last data point.
			value := timeSeries.Points[len(timeSeries.Points)-1].GetValue().GetDoubleValue()
			tags := make(map[string]string, len(includeLabels)+1)
			for _, label := range includeLabels {
				tags[label] = timeSeries.Resource.Labels[label]
			}
			tags["project"] = project
			metric := metrics2.GetFloat64Metric(measurement, tags)
			metric.Update(value)
			rv = append(rv, metric)
		}
	}
	return rv, nil
}

var _ MetricClient = &metricClientWrapper{}
