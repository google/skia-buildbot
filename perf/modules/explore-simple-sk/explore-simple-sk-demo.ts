import './index';
import '../../../elements-sk/modules/error-toast-sk';
import { setUpExploreDemoEnv } from '../common/test-util';
import fetchMock from 'fetch-mock';
import { BENCHMARK, BOT, SUBTEST_1, SUBTEST_2, TEST } from './test_data';
import { ParamSet } from '../../../infra-sk/modules/query';
import { $$ } from '../../../infra-sk/modules/dom';
import { ExploreSimpleSk } from './explore-simple-sk';

setUpExploreDemoEnv();

// Override defaults to enable test picker, which is required to attach the add-to-graph listener.
fetchMock.get(
  '/_/defaults/',
  {
    default_param_selections: null,
    default_url_values: { useTestPicker: 'false' },
    include_params: ['arch', 'os'],
  },
  { overwriteRoutes: true }
);

// Mock data for /_/nextParamList/
const mockNextParamListResponse = (opts: any) => {
  const body = JSON.parse(opts.body as string);
  const q: string = body.q;

  let paramset: ParamSet = {};
  let count = 0;

  if (q === '') {
    paramset = {
      benchmark: [BENCHMARK, 'other_benchmark'],
    };
    count = 100;
  } else if (q === `benchmark=${BENCHMARK}`) {
    paramset = {
      bot: [BOT, 'other_bot'],
    };
    count = 50;
  } else if (q === `benchmark=${BENCHMARK}&bot=${BOT}`) {
    paramset = {
      test: [encodeURIComponent(TEST), 'other_test'],
    };
    count = 20;
  } else if (q === `benchmark=${BENCHMARK}&bot=${BOT}&test=${encodeURIComponent(TEST)}`) {
    paramset = {
      subtest_1: [SUBTEST_1, SUBTEST_2],
    };
    count = 10;
  } else if (
    q ===
    `benchmark=${BENCHMARK}&bot=${BOT}&test=${encodeURIComponent(TEST)}&subtest_1=${SUBTEST_1}`
  ) {
    paramset = {}; // No more fields to select
    count = 5;
  } else {
    paramset = {};
    count = 0;
  }

  return { count, paramset };
};

fetchMock.post('/_/nextParamList/', mockNextParamListResponse, { overwriteRoutes: true });

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
  extra_links: null,
};

$$('#open_query_dialog')?.addEventListener('click', () => {
  document.querySelectorAll<ExploreSimpleSk>('explore-simple-sk').forEach((ele) => {
    ele.useTestPicker = false;
    ele.render();
  });
});
