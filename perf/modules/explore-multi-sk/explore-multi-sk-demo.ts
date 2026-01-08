import './index';
import '../../../elements-sk/modules/error-toast-sk';
import { setUpExploreDemoEnv } from '../common/test-util';
import fetchMock from 'fetch-mock';

setUpExploreDemoEnv();

// Override defaults to enable test picker, which is required to attach the add-to-graph listener.
fetchMock.get(
  '/_/defaults/',
  {
    default_param_selections: null,
    default_url_values: { useTestPicker: 'true' },
    include_params: ['arch', 'os'],
  },
  { overwriteRoutes: true }
);

window.perf = {
  dev_mode: false,
  instance_url: '',
  instance_name: 'chrome-perf-demo',
  header_image_url: '',
  commit_range_url: '',
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
  trace_format: 'chrome',
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

customElements.whenDefined('explore-multi-sk').then(() => {
  document
    .querySelector('h1')!
    .insertAdjacentElement('afterend', document.createElement('explore-multi-sk'));
});
