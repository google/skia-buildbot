import { Anomaly, CommitNumber } from '../json';

export const test_name =
  'ChromiumPerf/win-11-perf/webrtc/cpuTimeMetric_duration_std/multiple_peerconnections';

export const test_name_story = 'multiple_peerconnections';

export const new_test_name =
  'ChromiumPerf/win-10-perf/jetstream2/stanford-crypto-aes.Average/JetStream2';
export const y_value = 100;
export const commit_position = CommitNumber(12345);
export const bugId = 15423;

export const dummyAnomaly = (bugId: number): Anomaly => ({
  id: '1',
  test_path: '',
  bug_id: bugId,
  start_revision: 12345,
  end_revision: 12347,
  is_improvement: false,
  recovered: true,
  state: '',
  statistic: '',
  units: '',
  degrees_of_freedom: 0,
  median_before_anomaly: 75.209091,
  median_after_anomaly: 100.5023,
  p_value: 0,
  segment_size_after: 0,
  segment_size_before: 0,
  std_dev_before_anomaly: 0,
  t_statistic: 0,
  subscription_name: '',
  bug_component: '',
  bug_labels: [],
  bug_cc_emails: [],
  bisect_ids: [],
});
