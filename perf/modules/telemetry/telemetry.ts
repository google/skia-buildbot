/**
 * @fileoverview This file defines a Telemetry class for sending frontend metrics.
 * A singleton instance is exported for application-wide use. These metrics are
 * used to track user interactions and performance.
 *
 * Metrics are buffered on the frontend for 5 seconds before being sent in batches
 * to the `/_/fe_telemetry` endpoint. This reduces network traffic. Any pending metrics
 * are also sent when the page visibility changes to 'hidden' (e.g., when the user
 * navigates away or closes the tab) to prevent data loss.
 *
 * To add a new counter metric:
 * 1. Add the metric name to the `CountMetric` enum.
 * 2. Call `telemetry.increaseCounter()` with the new metric name and optional tags.
 *
 * To add a new summary metric:
 * 1. Add the metric name to the `SummaryMetric` enum.
 * 2. Call `telemetry.recordSummary()` with the new metric name, value, and optional tags.
 */
interface FrontendMetric {
  metric_name: string;
  metric_value: number;
  tags: { [key: string]: string };
  metric_type: 'counter' | 'summary';
}

export enum CountMetric {
  // go/keep-sorted start
  DataFetchFailure = 'fe_data_fetch_failure',
  MultiGraphVisit = 'fe_multi_graph_page_visit',
  ReportPageVisit = 'fe_report_page_visit',
  SIDRequiringActionTaken = 'fe_sid_requiring_action_taken',
  ExistingBugDialogSkBugIdUsedAsAnomalyKey = 'fe_exisitng_dialog_sk_bug_id_used_as_anomaly_key',
  TriageActionTaken = 'fe_triage_action_taken',
  // go/keep-sorted end
}

export enum SummaryMetric {
  // go/keep-sorted start
  GoogleGraphPlotTime = 'fe_google_graph_plot_time_s',
  MultiGraphDataLoadTime = 'fe_multi_graph_data_load_time_s',
  ReportAnomaliesTableLoadTime = 'fe_report_anomalies_table_load_time_s',
  ReportChartContainerLoadTime = 'fe_report_chart_container_load_time_s',
  ReportGraphChunkLoadTime = 'fe_report_graph_chunk_load_time_s',
  ReportPageLoadTime = 'fe_report_page_load_time_s',
  SingleGraphLoadTime = 'fe_single_graph_load_time_s',
  // go/keep-sorted end
}

class Telemetry {
  private static readonly BUFFER_FLUSH_INTERVAL_MS = 5000; // 5 seconds

  private static readonly MAX_BUFFER_SIZE = 1000; // Max 1000 metrics in buffer

  private metricsBuffer: FrontendMetric[] = [];

  private timerId: number | null = null;

  constructor() {
    // When the page visibility changes, flush the buffer. This helps ensure we
    // capture metrics before the user navigates away or closes the tab.
    document.addEventListener('visibilitychange', () => {
      if (document.visibilityState === 'hidden') {
        if (this.timerId) {
          clearTimeout(this.timerId);
          this.timerId = null;
        }
        this.sendBufferedMetrics();
      }
    });
  }

  // Flushes the metrics buffer by sending the data to the telemetry endpoint.
  private async sendBufferedMetrics() {
    if (this.metricsBuffer.length === 0) {
      return;
    }

    const metricsToSend = [...this.metricsBuffer];
    this.metricsBuffer.length = 0;

    try {
      await fetch('/_/fe_telemetry', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(metricsToSend),
      });
    } catch (e) {
      console.error(e, 'Failed to send frontend metrics:', metricsToSend);
      this.queueMetrics(metricsToSend);
    }
  }

  private queueMetric(metric: FrontendMetric) {
    this.queueMetrics([metric]);
  }

  private queueMetrics(metrics: FrontendMetric[]) {
    for (const m of metrics) {
      if (this.metricsBuffer.length >= Telemetry.MAX_BUFFER_SIZE) {
        console.warn('Frontend metrics buffer full, removing oldest metric to make space.');
        this.metricsBuffer.shift(); // Remove the oldest metric (FIFO)
      }
      this.metricsBuffer.push(m);
    }

    if (!this.timerId) {
      this.timerId = window.setTimeout(() => {
        this.sendBufferedMetrics();
        this.timerId = null;
      }, Telemetry.BUFFER_FLUSH_INTERVAL_MS);
    }
  }

  increaseCounter(metricName: CountMetric, tags = {}) {
    this.queueMetric({
      metric_name: metricName,
      metric_value: 1,
      tags: tags,
      metric_type: 'counter',
    });
  }

  recordSummary(metricName: SummaryMetric, val: number, tags = {}) {
    this.queueMetric({
      metric_name: metricName,
      metric_value: val,
      tags: tags,
      metric_type: 'summary',
    });
  }

  // The following are exposed for testing purposes.
  _forTesting = {
    reset: () => {
      this.metricsBuffer.length = 0;
      if (this.timerId) {
        clearTimeout(this.timerId);
        this.timerId = null;
      }
    },
    getBuffer: () => this.metricsBuffer,
    MAX_BUFFER_SIZE: Telemetry.MAX_BUFFER_SIZE,
  };
}

export const telemetry = new Telemetry();
