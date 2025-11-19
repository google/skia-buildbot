import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import { TemplateResult, html } from 'lit/html.js';
import {
  Anomaly,
  AnomalyMap,
  ColumnHeader,
  CommitNumber,
  DataFrame,
  ReadOnlyParamSet,
  TimestampSeconds,
  Trace,
  TraceSet,
} from '../json';
import { AnomalyData } from '../common/anomaly-data';
import { AnomalySk, getAnomalyDataMap } from './anomaly-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

const dummyAnomaly = (): Anomaly => ({
  id: '0',
  test_path: '',
  bug_id: -1,
  start_revision: 0,
  end_revision: 3,
  is_improvement: false,
  recovered: true,
  state: '',
  statistic: '',
  units: '',
  degrees_of_freedom: 0,
  median_before_anomaly: 0,
  median_after_anomaly: 0,
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

describe('getAnomalyDataMap', () => {
  const header: ColumnHeader[] = [
    {
      offset: CommitNumber(99),
      timestamp: TimestampSeconds(0),
      author: '',
      hash: '',
      message: '',
      url: '',
    },
    {
      offset: CommitNumber(100),
      timestamp: TimestampSeconds(0),
      author: '',
      hash: '',
      message: '',
      url: '',
    },
    {
      offset: CommitNumber(101),
      timestamp: TimestampSeconds(0),
      author: '',
      hash: '',
      message: '',
      url: '',
    },
  ];
  const traceset = TraceSet({
    traceA: Trace([5, 5, 15]),
    traceB: Trace([1, 1, 4]),
  });
  const dataframe: DataFrame = {
    traceset: traceset,
    header: header,
    skip: 0,
    paramset: ReadOnlyParamSet({}),
    traceMetadata: [],
  };
  const anomalyA: Anomaly = dummyAnomaly();
  const anomalyB: Anomaly = dummyAnomaly();
  const anomalymap: AnomalyMap = {
    traceA: { 101: anomalyA },
    traceB: { 101: anomalyB },
  };
  const expectedAnomalyDataMap: { [key: string]: AnomalyData[] } = {
    traceA: [
      {
        x: 2,
        y: 15,
        anomaly: anomalyA,
        highlight: false,
      },
    ],
    traceB: [
      {
        x: 2,
        y: 4,
        anomaly: anomalyB,
        highlight: false,
      },
    ],
  };
  it('returns two traces with one anomaly each', () => {
    const anomalyDataMap = getAnomalyDataMap(dataframe.traceset, dataframe.header!, anomalymap, []);
    assert.deepEqual(anomalyDataMap, expectedAnomalyDataMap);
  });
  it('maps anomaly to the next commit if exact match not available', () => {
    const columnHeader: ColumnHeader = {
      offset: CommitNumber(103),
      timestamp: TimestampSeconds(0),
      author: '',
      hash: '',
      message: '',
      url: '',
    };
    dataframe.header?.push(columnHeader);
    dataframe.traceset.traceA.push(200);
    // Add anomaly that does not have a commit in the header.
    const anomalymap = { traceA: { 102: anomalyA } };
    const dataMap = getAnomalyDataMap(dataframe.traceset, dataframe.header!, anomalymap, []);
    const expectedAnomalyMap: { [key: string]: AnomalyData[] } = {
      traceA: [
        {
          x: 3,
          y: 200,
          anomaly: anomalyA,
          highlight: false,
        },
      ],
    };
    assert.deepEqual(dataMap, expectedAnomalyMap);
  });
});

describe('formatPercentage', () => {
  it('returns percentage with a positive sign', () => {
    const formattedPerc = AnomalySk.formatPercentage(72);
    assert.equal(formattedPerc, `+72`);
  });

  it('returns percentage with a negative sign', () => {
    const formattedPerc = AnomalySk.formatPercentage(-72);
    assert.equal(formattedPerc, `-72`);
  });
});

describe('formatRevisionRange', () => {
  const newInstance = setUpElementUnderTest<AnomalySk>('anomaly-sk');
  let anomalySk: AnomalySk;

  const dummyCommits = [
    {
      offset: 64809,
      hash: '3b8de1058a896b613b451db1b6e2b28d58f64a4a',
      ts: 1676307170,
      author: 'Joe Gregorio \u003cjcgregorio@google.com\u003e',
      message: 'Add -prune to gazelle_update_repo run of gazelle.',
      url: 'https://skia.googlesource.com/skia/+show/3b8de1058a896b613b451db1b6e2b28d58f64a4a',
    },
    {
      offset: 64811,
      hash: '9039c60688c9511f9a553cd2443e412f68b5a107',
      ts: 1676308195,
      author: 'Jim Van Verth \u003cjvanverth@google.com\u003e',
      message: '[graphite] Add Dawn Windows job.',
      url: 'https://skia.googlesource.com/skia/+show/9039c60688c9511f9a553cd2443e412f68b5a107',
    },
  ];
  fetchMock.post('/_/cid/', () => ({
    commitSlice: dummyCommits,
    logEntry: `commit 3b8de1058a896b613b451db1b6e2b28d58f64a4a\nAuthor: Joe Gregorio \
    \u003cjcgregorio@google.com\u003e\nDate:   Mon Feb 13 10:20:19 2023 -0500\n\n    Add \
    -prune to gazelle_update_repo run of gazelle.\n    \n    Bug: b/269015892\n    \
    Change-Id: Iafd3c63e2e952ce1b95b52e56fb6d93a9410f69c\n    \
    Reviewed-on: https://skia-review.googlesource.com/c/skia/+/642338\n    \
    Reviewed-by: Leandro Lovisolo \u003clovisolo@google.com\u003e\n    \
    Commit-Queue: Joe Gregorio \u003cjcgregorio@google.com\u003e\n',`,
  }));

  beforeEach(() => {
    anomalySk = newInstance();
    anomalySk.anomaly = dummyAnomaly();

    window.perf = {
      instance_url: '',
      instance_name: 'chrome-perf-test',
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
  });

  it('does not change revision range if anomaly is null', async () => {
    const originalRevision = anomalySk.revision;
    anomalySk.anomaly = null;

    await anomalySk.formatRevisionRange();

    const newRevision = anomalySk.revision;

    assert.deepEqual(newRevision, originalRevision);
  });

  it('sets only revision text if commit range url missing', async () => {
    window.perf.commit_range_url = '';

    await anomalySk.formatRevisionRange();

    const revision = anomalySk.revision;
    const startRev = dummyAnomaly().start_revision;
    const endRev = dummyAnomaly().end_revision;
    const expected: TemplateResult = html`${startRev} - ${endRev}`;

    assert.deepEqual(revision, expected);
  });

  it('sets revision range url and text', async () => {
    await anomalySk.formatRevisionRange();
    const revision = anomalySk.revision;

    const url = window.perf.commit_range_url
      .replace('{begin}', dummyCommits[0].hash)
      .replace('{end}', dummyCommits[1].hash);
    const startRev = dummyAnomaly().start_revision;
    const endRev = dummyAnomaly().end_revision;
    const expected: TemplateResult = html`<a href="${url}" target=_blank>${startRev} - ${endRev}</td>`;

    assert.deepEqual(revision, expected);
  });
});
