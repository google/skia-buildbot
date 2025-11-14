import fetchMock from 'fetch-mock';
import { $$ } from '../../../infra-sk/modules/dom';
import { MISSING_DATA_SENTINEL } from '../const/const';
import { CommitRangeSk } from './commit-range-sk';

import './index';
import { CommitNumber, TimestampSeconds } from '../json';

window.perf = {
  dev_mode: false,
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
  logEntry:
    'commit 3b8de1058a896b613b451db1b6e2b28d58f64a4a\nAuthor: Joe Gregorio \u003cjcgregorio@google.com\u003e\nDate:   Mon Feb 13 10:20:19 2023 -0500\n\n    Add -prune to gazelle_update_repo run of gazelle.\n    \n    Bug: b/269015892\n    Change-Id: Iafd3c63e2e952ce1b95b52e56fb6d93a9410f69c\n    Reviewed-on: https://skia-review.googlesource.com/c/skia/+/642338\n    Reviewed-by: Leandro Lovisolo \u003clovisolo@google.com\u003e\n    Commit-Queue: Joe Gregorio \u003cjcgregorio@google.com\u003e\n',
}));

window.customElements.whenDefined('commit-range-sk').then(async () => {
  const ele = document.querySelector<CommitRangeSk>('commit-range-sk')!;
  ele.trace = [12, MISSING_DATA_SENTINEL, 13];
  ele.commitIndex = 2;
  ele.header = [
    {
      offset: CommitNumber(64809),
      timestamp: TimestampSeconds(0),
      author: '',
      hash: '',
      message: '',
      url: '',
    },
    {
      offset: CommitNumber(64810),
      timestamp: TimestampSeconds(0),
      author: '',
      hash: '',
      message: '',
      url: '',
    },
    {
      offset: CommitNumber(64811),
      timestamp: TimestampSeconds(0),
      author: '',
      hash: '',
      message: '',
      url: '',
    },
  ];
  await ele.recalcLink();
  $$<HTMLPreElement>('#url')!.textContent = ele.querySelector('a')!.href;
});
