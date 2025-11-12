import './index';
import { assert } from 'chai';
import { ChartTooltipSk } from './chart-tooltip-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { Anomaly, CommitNumber } from '../json';

describe('chart-tooltip-sk', () => {
  const newInstance = setUpElementUnderTest<ChartTooltipSk>('chart-tooltip-sk');

  let element: ChartTooltipSk;
  beforeEach(() => {
    // element = newInstance((el: ChartTooltipSk) => {
    //   // Place here any code that must run after the element is instantiated but
    //   // before it is attached to the DOM (e.g. property setter calls,
    //   // document-level event listeners, etc.).
    // });

    window.perf = {
      instance_url: '',
      instance_name: 'chrome-perf-test',
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
      bug_host_url: 'https://example.bug.url',
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

    element = newInstance();
  });

  const test_name =
    'ChromiumPerf/win-11-perf/webrtc/cpuTimeMetric_duration_std/multiple_peerconnections';
  const y_value = 100;
  const commit_position = CommitNumber(12345);
  const bugId = 15423;

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

  describe('set fields', () => {
    it('anomalies should be set', () => {
      element.load(
        1,
        test_name,
        '',
        'ms',
        y_value,
        new Date(),
        commit_position,
        0,
        dummyAnomaly(12345),
        null,
        null,
        false,
        null,
        () => {},
        undefined
      );
      assert.equal(element.test_name, test_name);
      assert.equal(element.y_value, y_value);
      assert.equal(element.commit_position, commit_position);
      assert.isNotNull(element.anomaly);
    });

    it('user issue should be set', () => {
      element.load(
        1,
        test_name,
        '',
        'ms',
        y_value,
        new Date(),
        commit_position,
        bugId,
        dummyAnomaly(12345),
        null,
        null,
        false,
        null,
        () => {},
        undefined
      );
      assert.equal(element.test_name, test_name);
      assert.equal(element.y_value, y_value);
      assert.equal(element.commit_position, commit_position);
      assert.equal(element.bug_id, bugId);
      assert.isNotNull(element.anomaly);
    });

    it('should be true when show_json_file_display is true', () => {
      window.perf.show_json_file_display = true;
      element = newInstance();
      element.load(
        1,
        test_name,
        '',
        'ms',
        y_value,
        new Date(),
        commit_position,
        0,
        dummyAnomaly(12345),
        null,
        null,
        false,
        null,
        () => {},
        undefined
      );
      element.loadJsonResource(commit_position, test_name);

      assert.isTrue(element.json_source);
      assert.equal(element.jsonSourceDialog!.cid, commit_position);
      assert.equal(element.jsonSourceDialog!.traceid, test_name);
    });
  });
});
