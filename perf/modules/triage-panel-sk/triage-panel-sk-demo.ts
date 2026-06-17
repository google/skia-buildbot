import './index';
import { TriagePanelSk } from './triage-panel-sk';
import { Anomaly } from '../json';

const mockAnomalies: Anomaly[] = [
  {
    id: '100',
    test_path: 'Master/Bot/Benchmark/Story/Metric1',
    bug_id: 0,
    start_revision: 1729647389,
    end_revision: 1739647389,
    display_commit_number: 1739647389,
    is_improvement: false,
    recovered: false,
    state: '',
    statistic: '',
    units: 'ms',
    degrees_of_freedom: 0,
    median_before_anomaly: 10,
    median_after_anomaly: 20,
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
  },
];

const panel = document.querySelector<TriagePanelSk>('triage-panel-sk');
if (panel) {
  panel.anomalies = mockAnomalies;
}
