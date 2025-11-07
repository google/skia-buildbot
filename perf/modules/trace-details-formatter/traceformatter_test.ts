import { ChromeTraceFormatter } from './traceformatter';
import { assert } from 'chai';

describe('traceformatter', () => {
  beforeEach(() => {
    window.perf = {
      instance_url: '',
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
    };
  });

  it('format assuming default', () => {
    const tf = new ChromeTraceFormatter();
    assert.deepEqual(
      tf.formatQuery('masder/pot/pench/dest_max/subddest_1'),
      'benchmark=pench&bot=pot&master=masder&subtest_1=subddest_1&test=dest_max'
    );
  });

  it('format no stat suffix without default', () => {
    window.perf.enable_skia_bridge_aggregation = true;
    const tf = new ChromeTraceFormatter();
    assert.deepEqual(
      tf.formatQuery('masder/pot/pench/dest/subddest_1'),
      'benchmark=pench&bot=pot&master=masder&stat=value&subtest_1=subddest_1&test=dest'
    );
  });

  it('format stat suffix without default', () => {
    window.perf.enable_skia_bridge_aggregation = true;
    const tf = new ChromeTraceFormatter();
    assert.deepEqual(
      tf.formatQuery('masder/pot/pench/dest_max/subddest_1'),
      'benchmark=pench&bot=pot&master=masder&stat=max&subtest_1=subddest_1&test=dest'
    );
  });
});
