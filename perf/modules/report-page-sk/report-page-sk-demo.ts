import './index';
import { $$ } from '../../../infra-sk/modules/dom';
import { ReportPageSk } from './report-page-sk';
import fetchMock from 'fetch-mock';
import { setUpExploreDemoEnv, anomalyTable, normalTracesResponse } from '../common/test-util';

anomalyTable[0].test_path = 'ChromiumPerf/mac-m1_mini_2020-perf/jetstream2/Babylon.First';

setUpExploreDemoEnv();

fetchMock.config.overwriteRoutes = true;

const TRACE_KEY = ',arch=arm,os=Android,';

const filteredResults = {
  dataframe: {
    ...normalTracesResponse.results.dataframe,
    traceset: {
      [TRACE_KEY]: normalTracesResponse.results.dataframe.traceset[TRACE_KEY],
    },
  },
  anomalymap: {
    [TRACE_KEY]: normalTracesResponse.results.anomalymap![TRACE_KEY],
  },
  ticks: normalTracesResponse.results.ticks,
  skps: normalTracesResponse.results.skps,
  msg: normalTracesResponse.results.msg,
  display_mode: normalTracesResponse.results.display_mode,
};

fetchMock.post('/_/frame/start', {
  results: filteredResults,
  status: 'Finished',
  messages: [],
  url: '',
});

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
  extra_links: null,
};

$$('#load-anomalies')?.addEventListener('click', () => {
  const ele = document.querySelector('report-page-sk') as ReportPageSk;
  ele.fetchAnomalies();
});

customElements.whenDefined('report-page-sk').then(() => {
  document
    .querySelector('h1')!
    .insertAdjacentElement('afterend', document.createElement('report-page-sk'));
});
