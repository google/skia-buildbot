import './index';
import fetchMock from 'fetch-mock';
import { $$ } from '../../../infra-sk/modules/dom';
import { ReportPageSk } from './report-page-sk';

fetchMock.post('/_/group_report', async () => {
  return {
    anomalyList: anomaly_table,
  };
});

window.perf = {
  commit_range_url: 'http://example.com/range/{begin}/{end}',
  key_order: ['config'],
  demo: true,
  radius: 7,
  num_shift: 10,
  interesting: 25,
  step_up_only: false,
  display_group_by: true,
  hide_list_of_commits_on_explore: false,
  notifications: 'none',
  fetch_chrome_perf_anomalies: false,
  feedback_url: '',
  chat_url: '',
  help_url_override: '',
  trace_format: '',
  need_alert_action: false,
  bug_host_url: 'b',
  git_repo_url: '',
  keys_for_commit_range: [],
  keys_for_useful_links: [],
  skip_commit_detail_display: false,
  image_tag: 'fake-tag',
};

const anomaly_table = [
  {
    id: 1,
    test_path: 'ChromePerf/linux-perf/Speed/Total/Jet',
    bug_id: 12345,
    start_revision: 1234,
    end_revision: 1239,
    is_improvement: false,
    recovered: true,
    state: '',
    statistic: '',
    units: 'delta',
    degrees_of_freedom: 0,
    median_before_anomaly: 75.209091,
    median_after_anomaly: 100.5023,
    p_value: 0,
    segment_size_after: 0,
    segment_size_before: 0,
    std_dev_before_anomaly: 0,
    t_statistic: 0,
    subscription_name: '',
    bug_component: 'Component A',
    bug_labels: [],
    bug_cc_emails: [],
    bisect_ids: [],
  },
  {
    id: 2,
    test_path: 'ChromePerf/mac-m1/Motion/Score/Motion',
    bug_id: -1,
    start_revision: 1234,
    end_revision: 1234,
    is_improvement: false,
    recovered: true,
    state: '',
    statistic: '',
    units: 'ms',
    degrees_of_freedom: 0,
    median_before_anomaly: 1.345,
    median_after_anomaly: 2.403,
    p_value: 0,
    segment_size_after: 0,
    segment_size_before: 0,
    std_dev_before_anomaly: 0,
    t_statistic: 0,
    subscription_name: '',
    bug_component: 'Component B',
    bug_labels: [],
    bug_cc_emails: [],
    bisect_ids: [],
  },
  {
    id: 3,
    test_path: 'ChromePerf/mac-m1/Motion/Score/Motion',
    bug_id: 12345,
    start_revision: 34567,
    end_revision: 34569,
    is_improvement: true,
    recovered: true,
    state: '',
    statistic: '',
    units: 'ns',
    degrees_of_freedom: 0,
    median_before_anomaly: 72.209091,
    median_after_anomaly: 73.5023,
    p_value: 0,
    segment_size_after: 0,
    segment_size_before: 0,
    std_dev_before_anomaly: 0,
    t_statistic: 0,
    subscription_name: '',
    bug_component: 'Component B',
    bug_labels: [],
    bug_cc_emails: [],
    bisect_ids: [],
  },
  {
    id: 4,
    test_path: 'ChromePerf/mac-m1/Motion/Score/Motion',
    bug_id: -1,
    start_revision: 1234,
    end_revision: 1239,
    is_improvement: false,
    recovered: true,
    state: '',
    statistic: '',
    units: 'ms',
    degrees_of_freedom: 0,
    median_before_anomaly: 75.209091,
    median_after_anomaly: 100.5023,
    p_value: 0,
    segment_size_after: 0,
    segment_size_before: 0,
    std_dev_before_anomaly: 0,
    t_statistic: 0,
    subscription_name: '',
    bug_component: 'Component C',
    bug_labels: [],
    bug_cc_emails: [],
    bisect_ids: [],
  },
];

$$('#load-anomalies')?.addEventListener('click', () => {
  const ele = document.querySelector('report-page-sk') as ReportPageSk;
  ele.fetchAnomalies();
});
