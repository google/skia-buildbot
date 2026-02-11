import './chart-tooltip-sk';
import { ChartTooltipSk } from './chart-tooltip-sk';
import { commit_position, dummyAnomaly, new_test_name, test_name, y_value } from './test_data';
import { $$ } from '../../../infra-sk/modules/dom';
import fetchMock from 'fetch-mock';
import { CIDHandlerResponse, CommitNumber } from '../json';

const cidHandlerResponse: CIDHandlerResponse = {
  commitSlice: [
    {
      offset: commit_position,
      hash: '1234567890abcdef',
      ts: 1676400000,
      author: 'user@example.com',
      message: 'Test commit message',
      url: 'http://example.com/commit/12345',
      body: 'Test commit body',
    },
    {
      offset: CommitNumber(12346),
      hash: 'fedcba0987654321',
      ts: 1676400000,
      author: 'user@example.com',
      message: 'Test commit message 2',
      url: 'http://example.com/commit/12346',
      body: 'Test commit body 2',
    },
  ],
  logEntry: 'Test log entry',
};

fetchMock.post('/_/cid/', cidHandlerResponse);

// Mock data for the tooltip.
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
  feedback_url: '',
  fetch_anomalies_from_sql: false,
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
  extra_links: null,
};

$$('#load-initial-data')?.addEventListener('click', () => {
  document.querySelectorAll<ChartTooltipSk>('chart-tooltip-sk').forEach((ele) => {
    console.log('chart-tooltip-sk-demo.ts: load-initial-data');
    ele.load(
      1,
      test_name,
      ',arch=x86,config=8888,test=decode,units=kb,',
      'ms',
      y_value,
      new Date(),
      commit_position,
      0,
      null,
      null,
      null,
      false,
      null,
      () => {},
      undefined
    );
  });
});

$$('#reset-tooltip')?.addEventListener('click', () => {
  document.querySelectorAll<ChartTooltipSk>('chart-tooltip-sk').forEach((ele) => {
    console.log('chart-tooltip-sk-demo.ts: reset-tooltip');
    ele.reset();
  });
});

$$('#load-data-with-anomaly')?.addEventListener('click', () => {
  document.querySelectorAll<ChartTooltipSk>('chart-tooltip-sk').forEach((ele) => {
    console.log('chart-tooltip-sk-demo.ts: load-data-with-anomaly');
    ele.load(
      1,
      new_test_name,
      ',arch=x86,config=8888,test=decode,units=kb,',
      'ms',
      y_value,
      new Date(),
      commit_position,
      12345,
      dummyAnomaly(12345),
      null,
      null,
      false,
      null,
      () => {},
      undefined
    );
  });
});

$$('#load-data-without-anomaly')?.addEventListener('click', () => {
  document.querySelectorAll<ChartTooltipSk>('chart-tooltip-sk').forEach((ele) => {
    console.log('chart-tooltip-sk-demo.ts: load-data-without-anomaly');
    ele.load(
      1,
      new_test_name,
      ',arch=x86,config=8888,test=decode,units=kb,',
      'ms',
      y_value,
      new Date(),
      commit_position,
      0,
      null,
      null,
      null,
      false,
      null,
      () => {},
      undefined
    );
  });
});
