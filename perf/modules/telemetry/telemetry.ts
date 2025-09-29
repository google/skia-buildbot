/**
 * @fileoverview This file defines functions for sending frontend telemetry metrics.
 * These metrics are used to track user interactions and performance within the
 * application.
 *
 * To add a new counter metric:
 * 1. Add the metric name to the `CountMetricName` type.
 * 2. Call the `increaseCounter` function with the new metric name and optional tags.
 *
 * To add a new summary metric:
 * 1. Add the metric name to the `SummaryMetricName` type.
 * 2. Call the `recordSummary` function with the new metric name, value, and optional tags.
 */

interface FrontendMetric {
  metric_name: string;
  metric_value: number;
  tags: { [key: string]: string };
  metric_type: 'counter' | 'summary';
}

type CountMetricName =
  // Counts the specific triage actions users perform (e.g., filing a bug, ignoring an anomaly).
  'fe_triage_action_taken';

type SummaryMetricName =
  // Measures the time taken to fetch and process data when plotting new graphs.
  'fe_multi_graph_data_load_time_s';

export async function increaseCounter(metricName: CountMetricName, tags = {}) {
  sendMetrics([
    {
      metric_name: metricName as string,
      metric_value: 1,
      tags: tags,
      metric_type: 'counter',
    },
  ]);
}

export async function recordSummary(metricName: SummaryMetricName, val: number, tags = {}) {
  sendMetrics([
    {
      metric_name: metricName as string,
      metric_value: val,
      tags: tags,
      metric_type: 'summary',
    },
  ]);
}

async function sendMetrics(metrics: FrontendMetric[]) {
  fetch('/_/fe_telemetry', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(metrics),
  }).catch((e) => {
    console.error(e, 'Failed to send frontend metrics:', metrics);
  });
}
