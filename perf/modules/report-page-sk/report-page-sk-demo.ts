import './index';
import { $$ } from '../../../infra-sk/modules/dom';
import { ReportPageSk } from './report-page-sk';

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
};

$$('#load-anomalies')?.addEventListener('click', () => {
  window.perf = window.perf;
  const ele = document.querySelector('report-page-sk') as ReportPageSk;
  ele.fetchAnomalies();
});
