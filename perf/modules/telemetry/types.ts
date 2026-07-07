/**
 * @fileoverview This file defines shared types and enums for the telemetry module.
 * It helps break circular dependencies between telemetry and other modules.
 */

export interface TelemetryErrorOptions {
  source?: string;
  errorCode?: string;
  endpoint?: string;
  method?: string;
  url?: string;
  stack?: string;
  countMetricSource?: CountMetric;
}

export enum CountMetric {
  // go/keep-sorted start
  // AnomalyDataRaceTraceNotFound - Anomalies miss on graph due to race condition,
  // trace was not there yet during anomalies assigment.
  AnomalyDataRaceTraceNotFound = 'fe_anomaly_data_race_trace_not_found',
  ConfirmedRegressionInvalidPayload = 'fe_confirmed_regression_invalid_payload',
  DataFetchFailure = 'fe_data_fetch_failure',
  ExistingBugDialogSkBugIdUsedAsAnomalyKey = 'fe_exisitng_dialog_sk_bug_id_used_as_anomaly_key',
  ExploreMultiV2Visit = 'fe_explore_multi_v2_page_visit_count',
  FrontendErrorReported = 'fe_errors_reported_count',
  MultiGraphVisit = 'fe_multi_graph_page_visit_count',
  ReportPageVisit = 'fe_report_page_visit_count',
  SIDRequiringActionTaken = 'fe_sid_requiring_action_taken',
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
