import './index';
import fetchMock from 'fetch-mock';

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

fetchMock.get('/_/subscriptions', () => [
  {
    name: 'Sheriff Config 1',
    revision: 'rev1',
    bug_labels: ['A', 'B'],
    hotlists: ['C', 'D'],
    bug_components: 'Component1>Subcomponent1',
    bug_priority: 1,
    bug_severity: 2,
    bug_cc_emails: ['abcd@efg.com', '1234@567.com'],
    contact_email: 'test@owner.com',
  },
  {
    name: 'Sheriff Config 2',
    revision: 'rev2',
    bug_labels: ['1', '2'],
    hotlists: ['3', '4'],
    bug_components: 'Component2>Subcomponent2',
    bug_priority: 1,
    bug_severity: 2,
    bug_cc_emails: ['abcd@efg.com', '1234@567.com'],
    contact_email: 'test@owner.com',
  },
  {
    name: 'Sheriff Config 3',
    revision: 'rev3',
    bug_labels: ['1', '2'],
    hotlists: ['3', '4'],
    bug_components: 'Component3>Subcomponent3',
    bug_priority: 1,
    bug_severity: 2,
    bug_cc_emails: ['abcd@efg.com', '1234@567.com'],
    contact_email: 'test@owner.com',
  },
]);

fetchMock.get('/_/anomalies/sheriff_list', {
  sheriff_list: ['Sheriff Config 1', 'Sheriff Config 2', 'Sheriff Config 3'],
});

fetchMock.get(`/_/regressions?sub_name=Sheriff%20Config%201&limit=10&offset=0`, [
  {
    id: 'id1',
    commit_number: 1234,
    prev_commit_number: 1236,
    alert_id: 1,
    creation_time: '',
    median_before: 123,
    median_after: 135,
    is_improvement: true,
    cluster_type: 'high',
    frame: {
      dataframe: {
        paramset: {
          bot: ['bot1'],
          benchmark: ['benchmark1'],
          test: ['test1'],
          improvement_direction: ['up'],
        },
        traceset: {},
        header: null,
        skip: 1,
      },
      skps: [1],
      msg: '',
      anomalymap: null,
    },
    high: {
      centroid: null,
      shortcut: 'shortcut 1',
      param_summaries2: null,
      step_fit: {
        status: 'High',
        least_squares: 123,
        regression: 12,
        step_size: 345,
        turning_point: 1234,
      },
      step_point: null,
      num: 156,
      ts: 'test',
    },
  },
]);

fetchMock.get(`/_/regressions?sub_name=Sheriff%20Config%202&limit=10&offset=0`, [
  {
    id: 'id2',
    commit_number: 1235,
    prev_commit_number: 1237,
    alert_id: 1,
    creation_time: '',
    median_before: 123,
    median_after: 135,
    is_improvement: true,
    cluster_type: 'high',
    frame: {
      dataframe: {
        paramset: {
          bot: ['bot1'],
          benchmark: ['benchmark1'],
          test: ['test1'],
          improvement_direction: ['up'],
        },
        traceset: {},
        header: null,
        skip: 1,
      },
      skps: [1],
      msg: '',
      anomalymap: null,
    },
    high: {
      centroid: null,
      shortcut: 'shortcut 1',
      param_summaries2: null,
      step_fit: {
        status: 'High',
        least_squares: 123,
        regression: 12,
        step_size: 345,
        turning_point: 1234,
      },
      step_point: null,
      num: 156,
      ts: 'test',
    },
  },
]);

fetchMock.get(`/_/regressions?sub_name=Sheriff%20Config%203&limit=10&offset=0`, []);

document.querySelector('.component-goes-here')!.innerHTML =
  '<regressions-page-sk></regressions-page-sk>';
