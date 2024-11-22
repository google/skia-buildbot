import './index';
import fetchMock from 'fetch-mock';
import { $$ } from '../../../infra-sk/modules/dom';
import { Anomaly } from '../json';
import { AnomalySk } from './anomaly-sk';

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
  bug_host_url: '',
  git_repo_url: '',
  keys_for_commit_range: [],
  image_tag: 'fake-tag',
};

const dummyAnomaly = (bugId: number): Anomaly => ({
  id: 1,
  test_path: '',
  bug_id: bugId,
  start_revision: 1234,
  end_revision: 1239,
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
});

// The response to a POST of [64809, 64811] to /_/cid/.
fetchMock.post('/_/cid/', () => ({
  commitSlice: [
    {
      offset: 64809,
      hash: '3b8de1058a896b613b451db1b6e2b28d58f64a4a',
      ts: 1676307170,
      author: 'Joe Gregorio \u003cjcgregorio@google.com\u003e',
      message: 'Add -prune to gazelle_update_repo run of gazelle.',
      url: 'https://skia.googlesource.com/skia/+show/3b8de1058a896b613b451db1b6e2b28d58f64a4a',
    },
    {
      offset: 64811,
      hash: '9039c60688c9511f9a553cd2443e412f68b5a107',
      ts: 1676308195,
      author: 'Jim Van Verth \u003cjvanverth@google.com\u003e',
      message: '[graphite] Add Dawn Windows job.',
      url: 'https://skia.googlesource.com/skia/+show/9039c60688c9511f9a553cd2443e412f68b5a107',
    },
  ],
  logEntry: `commit 3b8de1058a896b613b451db1b6e2b28d58f64a4a\nAuthor: Joe Gregorio \
    \u003cjcgregorio@google.com\u003e\nDate:   Mon Feb 13 10:20:19 2023 -0500\n\n    Add \
    -prune to gazelle_update_repo run of gazelle.\n    \n    Bug: b/269015892\n    \
    Change-Id: Iafd3c63e2e952ce1b95b52e56fb6d93a9410f69c\n    \
    Reviewed-on: https://skia-review.googlesource.com/c/skia/+/642338\n    \
    Reviewed-by: Leandro Lovisolo \u003clovisolo@google.com\u003e\n    \
    Commit-Queue: Joe Gregorio \u003cjcgregorio@google.com\u003e\n',`,
}));

window.customElements.whenDefined('anomaly-sk').then(() => {
  $$<AnomalySk>('#good')!.anomaly = dummyAnomaly(12345);
  $$<AnomalySk>('#good-dark')!.anomaly = dummyAnomaly(12345);
  $$<AnomalySk>('#empty-bug')!.anomaly = dummyAnomaly(-1);
});
