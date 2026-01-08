import './index';
import { $$ } from '../../../infra-sk/modules/dom';
import { ReportPageSk } from './report-page-sk';
import { anomaly_table } from '../anomalies-table-sk/test_data';

window.perf = {
  instance_url: 'https://chrome-perf.corp.goog',
  instance_name: 'chrome-perf-demo',
  header_image_url: '',
  commit_range_url: 'https://chromium.googlesource.com/chromium/src/+log/{begin}..{end}',
  key_order: ['config'],
  demo: true,
  radius: 7,
  num_shift: 10,
  interesting: 25,
  step_up_only: false,
  display_group_by: false,
  hide_list_of_commits_on_explore: true,
  notifications: 'none',
  fetch_chrome_perf_anomalies: false,
  fetch_anomalies_from_sql: false,
  feedback_url: '',
  chat_url: '',
  help_url_override: '',
  trace_format: 'chrome',
  need_alert_action: false,
  bug_host_url: 'b',
  git_repo_url: 'https://chromium.googlesource.com/chromium/src',
  keys_for_commit_range: [],
  keys_for_useful_links: [],
  skip_commit_detail_display: false,
  image_tag: 'fake-tag',
  remove_default_stat_value: false,
  enable_skia_bridge_aggregation: false,
  show_json_file_display: false,
  always_show_commit_info: false,
  show_triage_link: false,
  show_bisect_btn: true,
  app_version: 'test-version',
  enable_v2_ui: false,
  dev_mode: false,
};

$$('#load-anomalies')?.addEventListener('click', () => {
  const ele = document.querySelector('report-page-sk') as ReportPageSk;
  ele.fetchAnomalies();
});

$$('#open-trending-icon')?.addEventListener('click', () => {
  document.querySelectorAll<ReportPageSk>('report-page-sk').forEach((ele) => {
    ele.fetchAnomalies();
    ele.anomaliesTable!.openMultiGraphUrl(
      anomaly_table[0],
      window.open(
        'http://localhost:46723/m/?' +
          'begin=1729042589&end=11739042589&request_type=0&shortcut=1&totalGraphs=1',
        '_blank'
      )
    );
  });
});
