import './index';
import fetchMock from 'fetch-mock';

import { Anomaly, ColumnHeader, CommitNumber, TimestampSeconds } from '../json';
import { ChartTooltipSk } from './chart-tooltip-sk';

window.perf = {
  instance_url: '',
  instance_name: 'chrome-perf-demo',
  header_image_url: '',
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
  fetch_anomalies_from_sql: false,
  feedback_url: '',
  chat_url: '',
  help_url_override: '',
  trace_format: '',
  need_alert_action: false,
  bug_host_url: '',
  git_repo_url: '',
  keys_for_commit_range: [],
  keys_for_useful_links: [],
  skip_commit_detail_display: false,
  image_tag: 'fake-tag',
  remove_default_stat_value: false,
  enable_skia_bridge_aggregation: false,
  show_json_file_display: false,
  always_show_commit_info: false,
  show_triage_link: true,
  show_bisect_btn: true,
  app_version: 'test-version',
  enable_v2_ui: false,
};

const dummyAnomaly = (bugId: number): Anomaly => ({
  id: '1',
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
  bisect_ids: [],
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
  ],
  logEntry: `commit 3b8de1058a896b613b451db1b6e2b28d58f64a4a\nAuthor: Joe Gregorio \
    \u003cjcgregorio@google.com\u003e\nDate:   Mon Feb 13 10:20:19 2023 -0500\n\n    Add \
    -prune to gazelle_update_repo run of gazelle.\n    \n    Bug: b/269015892\n    \
    Change-Id: Iafd3c63e2e952ce1b95b52e56fb6d93a9410f69c\n    \
    Reviewed-on: https://skia-review.googlesource.com/c/skia/+/642338\n    \
    Reviewed-by: Leandro Lovisolo \u003clovisolo@google.com\u003e\n    \
    Commit-Queue: Joe Gregorio \u003cjcgregorio@google.com\u003e\n',`,
}));

fetchMock.get('/_/login/status', {
  email: 'someone@example.org',
  roles: ['editor'],
});

fetchMock.post('/_/details/?results=false', () => ({
  version: 1,
  git_hash: '04cfbf7e7ce2139ed3fd58a368e80f72a967d57e',
  key: {
    arch: 'x86',
    config: '8888',
  },
  results: null,
  links: {
    link1: 'http://google.com',
  },
}));

const renderTooltips = () => {
  document
    .querySelectorAll<ChartTooltipSk>('.buganizerTooltipContainer > chart-tooltip-sk')
    .forEach((tooltip) => {
      tooltip!.moveTo({ x: 0, y: 0 });
      const c: ColumnHeader = {
        author: 'a@b.com',
        hash: 'a1b2c3',
        message: 'Commit message',
        offset: CommitNumber(12345),
        timestamp: 1234566778 as TimestampSeconds,
        url: '',
      };
      tooltip!.load(
        1,
        'Tooltip with buganizer Id',
        ',arch=x86,config=8888,test=encode,units=kb,',
        'ms',
        100,
        new Date(),
        CommitNumber(12345),
        88123,
        dummyAnomaly(12345),
        null,
        c,
        true,
        null,
        () => {},
        undefined
      );
    });
};

renderTooltips();
