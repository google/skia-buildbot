import './chart-tooltip-sk';
import { ChartTooltipSk } from './chart-tooltip-sk';
import { CommitRangeSk } from '../commit-range-sk/commit-range-sk';
import { commit_position, dummyAnomaly, new_test_name, test_name, y_value } from './test_data';
import { $$ } from '../../../infra-sk/modules/dom';
import fetchMock from 'fetch-mock';
import { CIDHandlerResponse, CommitNumber, TimestampSeconds } from '../json';

const cidHandlerResponse: CIDHandlerResponse = {
  commitSlice: [
    {
      offset: CommitNumber(12345),
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
    ele.moveTo({ x: 100, y: 200 });
    ele.load(
      1,
      test_name,
      ',arch=x86,config=8888,test=decode,units=kb,',
      'ms',
      y_value,
      new Date(1678886400000), // use determenistic date for testing (Wed, 15 Mar 2023 13:20:00 GMT)
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
    // Move to null hides the tooltip.
    ele.moveTo(null);
    ele.reset();
  });
});

const createCommitRangeSk = async () => {
  const ele = new CommitRangeSk();
  ele.trace = [12, 13];
  ele.commitIndex = 1;
  ele.header = [
    {
      offset: CommitNumber(12345),
      timestamp: TimestampSeconds(0),
      author: '',
      hash: '',
      message: '',
      url: '',
    },
    {
      offset: CommitNumber(12346),
      timestamp: TimestampSeconds(0),
      author: '',
      hash: '',
      message: '',
      url: '',
    },
  ];
  await ele.recalcLink();
  return ele;
};

$$('#load-data-with-anomaly')?.addEventListener('click', () => {
  document.querySelectorAll<ChartTooltipSk>('chart-tooltip-sk').forEach(async (ele) => {
    console.log('chart-tooltip-sk-demo.ts: load-data-with-anomaly');
    // Move to a defined position to show the tooltip.
    ele.moveTo({ x: 100, y: 100 });
    const commitRange = await createCommitRangeSk();
    ele.load(
      /* index= */ 1,
      /* test_name= */ new_test_name,
      /* trace_name= */ ',arch=x86,config=8888,test=decode,units=kb,',
      /* unit_type= */ 'ms',
      /* y_value= */ y_value,
      /* date_value= */ new Date(1678886400000), // March 15, 2023 00:00:00 UTC
      /* commit_position= */ commit_position,
      /* bug_id= */ 0,
      /* anomaly= */ dummyAnomaly(12345),
      /* nudgeList= */ null,
      /* commit= */ null,
      /* tooltipFixed= */ false,
      /* commitRange= */ commitRange,
      /* closeButtonAction= */ () => {},
      /* color= */ undefined,
      /* user_id= */ undefined
    );
  });
});

$$('#load-data-without-anomaly')?.addEventListener('click', () => {
  document.querySelectorAll<ChartTooltipSk>('chart-tooltip-sk').forEach(async (ele) => {
    console.log('chart-tooltip-sk-demo.ts: load-data-without-anomaly');
    // Move to a defined position to show the tooltip.
    ele.moveTo({ x: 100, y: 100 });
    const commitRange = await createCommitRangeSk();
    ele.load(
      /* index= */ 1,
      /* test_name= */ new_test_name,
      /* trace_name= */ ',arch=x86,config=8888,test=decode,units=kb,',
      /* unit_type= */ 'ms',
      /* y_value= */ y_value,
      /* date_value= */ new Date(1678886400000), // March 15, 2023 00:00:00 UTC
      /* commit_position= */ commit_position,
      /* bug_id= */ 0,
      /* anomaly= */ null,
      /* nudgeList= */ null,
      /* commit= */ null,
      /* tooltipFixed= */ false,
      /* commitRange= */ commitRange,
      /* closeButtonAction= */ () => {},
      /* color= */ undefined,
      /* user_id= */ undefined
    );
  });
});
