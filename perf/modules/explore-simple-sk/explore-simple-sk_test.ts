/* eslint-disable dot-notation */
import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import {
  Anomaly,
  AnomalyMap,
  ColumnHeader,
  CommitNumber,
  DataFrame,
  FrameRequest,
  FrameResponse,
  QueryConfig,
  ReadOnlyParamSet,
  TimestampSeconds,
  Trace,
  TraceSet,
  progress,
} from '../json';
import { deepCopy } from '../../../infra-sk/modules/object';
import {
  calculateRangeChange,
  defaultPointSelected,
  ExploreSimpleSk,
  isValidSelection,
  PointSelected,
  selectionToEvent,
  CommitRange,
  GraphConfig,
  updateShortcut,
} from './explore-simple-sk';
import { timestampBounds, buildParamSet } from '../dataframe';
import { toParamSet, fromParamSet } from '../../../infra-sk/modules/query';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

fetchMock.config.overwriteRoutes = true;

const dummyAnomaly: Anomaly = {
  id: 0,
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
};

describe('calculateRangeChange', () => {
  const offsets: CommitRange = [100, 120] as CommitRange;

  it('finds a left range increase', () => {
    const zoom: CommitRange = [-1, 10] as CommitRange;
    const clampedZoom: CommitRange = [0, 10] as CommitRange;

    const ret = calculateRangeChange(zoom, clampedZoom, offsets);
    assert.isTrue(ret.rangeChange);

    // We shift the beginning of the range by RANGE_CHANGE_ON_ZOOM_PERCENT of
    // the total range.
    assert.deepEqual(ret.newOffsets, [90, 120]);
  });

  it('finds a right range increase', () => {
    const zoom: CommitRange = [0, 12] as CommitRange;
    const clampedZoom: CommitRange = [0, 10] as CommitRange;

    const ret = calculateRangeChange(zoom, clampedZoom, offsets);
    assert.isTrue(ret.rangeChange);

    // We shift the end of the range by RANGE_CHANGE_ON_ZOOM_PERCENT of the
    // total range.
    assert.deepEqual(ret.newOffsets, [100, 130]);
  });

  it('find an increase in the range in both directions', () => {
    const zoom: CommitRange = [-1, 11] as CommitRange;
    const clampedZoom: CommitRange = [0, 10] as CommitRange;

    const ret = calculateRangeChange(zoom, clampedZoom, offsets);
    assert.isTrue(ret.rangeChange);

    // We shift both the begin and end of the range by
    // RANGE_CHANGE_ON_ZOOM_PERCENT of the total range.
    assert.deepEqual(ret.newOffsets, [90, 130]);
  });

  it('find an increase in the range in both directions and clamps the offset', () => {
    const zoom: CommitRange = [-1, 11] as CommitRange;
    const clampedZoom: CommitRange = [0, 10] as CommitRange;
    const widerOffsets: CommitRange = [0, 100] as CommitRange;

    const ret = calculateRangeChange(zoom, clampedZoom, widerOffsets);
    assert.isTrue(ret.rangeChange);

    // We shift both the begin and end of the range by
    // RANGE_CHANGE_ON_ZOOM_PERCENT of the total range.
    assert.deepEqual(ret.newOffsets, [0, 150]);
  });

  it('does not find a range change', () => {
    const zoom: CommitRange = [0, 10] as CommitRange;
    const clampedZoom: CommitRange = [0, 10] as CommitRange;

    const ret = calculateRangeChange(zoom, clampedZoom, offsets);
    assert.isFalse(ret.rangeChange);
  });
});

describe('applyFuncToTraces', () => {
  window.perf = {
    radius: 2,
    key_order: null,
    num_shift: 50,
    interesting: 2,
    step_up_only: false,
    commit_range_url: '',
    demo: true,
    display_group_by: false,
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
  };

  // Create a common element-sk to be used by all the tests.
  const explore = document.createElement(
    'explore-simple-sk'
  ) as ExploreSimpleSk;
  document.body.appendChild(explore);

  const finishedBody: progress.SerializedProgress = {
    status: 'Finished',
    messages: [],
    results: {},
    url: '',
  };

  it('applies the func to existing formulas', async () => {
    const startURL = '/_/frame/start';

    // We mock out a response that returns a Progress that is Finished
    // so we don't have to mock out any more responses, we are just checking
    // on what explore-sk sends in the POST request.
    fetchMock.post(startURL, finishedBody);

    // Add a formula we expect to be wrapped.
    explore['state'].formulas = ['shortcut("Xfoo")'];
    await explore['applyFuncToTraces']('iqrr');

    // Confirm we hit the mock.
    assert.isTrue(fetchMock.done());

    // Confirm the formula is wrapped in iqrr().
    const body = JSON.parse(
      fetchMock.lastOptions(startURL)?.body as unknown as string
    ) as any;
    assert.deepEqual(body.formulas, ['iqrr(shortcut("Xfoo"))']);
    fetchMock.restore();
  });
});

describe('PointSelected', () => {
  it('defaults to not having a name', () => {
    const p = defaultPointSelected();
    assert.isEmpty(p.name);
  });

  it('defaults to being invalid', () => {
    const p = defaultPointSelected();
    assert.isFalse(isValidSelection(p));
  });

  it('becomes a valid event if the commit appears in the header', () => {
    const header: ColumnHeader[] = [
      {
        offset: CommitNumber(99),
        timestamp: TimestampSeconds(0),
      },
      {
        offset: CommitNumber(100),
        timestamp: TimestampSeconds(0),
      },
      {
        offset: CommitNumber(101),
        timestamp: TimestampSeconds(0),
      },
    ];

    const p: PointSelected = {
      commit: CommitNumber(100),
      name: 'foo',
    };
    // selectionToEvent will look up the commit (aka offset) in header and
    // should return an event where the 'x' value is the index of the matching
    // ColumnHeader in 'header', i.e. 1.
    const e = selectionToEvent(p, header);
    assert.equal(e.detail.x, 1);
  });

  it('becomes an invalid event if the commit does not appear in the header', () => {
    const header: ColumnHeader[] = [
      {
        offset: CommitNumber(99),
        timestamp: TimestampSeconds(0),
      },
      {
        offset: CommitNumber(100),
        timestamp: TimestampSeconds(0),
      },
      {
        offset: CommitNumber(101),
        timestamp: TimestampSeconds(0),
      },
    ];

    const p: PointSelected = {
      commit: CommitNumber(102),
      name: 'foo',
    };
    // selectionToEvent will look up the commit (aka offset) in header and
    // should return an event where the 'x' value is -1 since the matching
    // ColumnHeader in 'header' doesn't exist.
    const e = selectionToEvent(p, header);
    assert.equal(e.detail.x, -1);
  });
});

describe('updateShortcut', () => {
  it('should return empty shortcut if graph configs are absent', async () => {
    const shortcut = await updateShortcut([]);
    assert.equal(shortcut, '');
  });

  it('should return shortcut for non empty graph list', async () => {
    const defaultConfig: QueryConfig = {
      default_param_selections: null,
      default_url_values: null,
      include_params: null,
    };

    const defaultBody = JSON.stringify(defaultConfig);
    fetchMock.get('path:/_/defaults/', {
      status: 200,
      body: defaultBody,
    });

    fetchMock.post('/_/count/', {
      count: 117,
      paramset: {},
    });

    fetchMock.get(/_\/initpage\/.*/, () => ({
      dataframe: {
        traceset: null,
        header: null,
        paramset: {},
        skip: 0,
      },
      ticks: [],
      skps: [],
      msg: '',
    }));

    fetchMock.postOnce('/_/shortcut/update', { id: '12345' });
    fetchMock.flush(true);

    const shortcut = await updateShortcut([
      { keys: '', queries: [], formulas: [] },
    ] as GraphConfig[]);

    assert.deepEqual(shortcut, '12345');
  });
});

describe('addToAnomalyMap', () => {
  it('should set fullAnomalyMap if null', () => {
    const explore =
      setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();
    assert.equal(explore.fullAnomalyMap, null);

    const anomalymap: AnomalyMap = {
      traceA: { 101: { ...dummyAnomaly } },
      traceB: { 101: { ...dummyAnomaly } },
    };

    explore.addToAnomalyMap(anomalymap);
    assert.deepEqual(explore.fullAnomalyMap, anomalymap);
  });

  it('should set fullAnomalyMap if empty', () => {
    const explore =
      setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();
    explore.fullAnomalyMap = {};
    assert.deepEqual(explore.fullAnomalyMap, {});

    const anomalymap: AnomalyMap = {
      traceA: { 101: { ...dummyAnomaly } },
      traceB: { 101: { ...dummyAnomaly } },
    };

    explore.addToAnomalyMap(anomalymap);
    assert.deepEqual(explore.fullAnomalyMap, anomalymap);
  });

  it('should update fullAnomalyMap if non-empty', () => {
    const explore =
      setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();

    const anomalymap: AnomalyMap = {
      traceA: { 101: { ...dummyAnomaly } },
      traceB: { 101: { ...dummyAnomaly } },
    };

    explore.fullAnomalyMap = anomalymap;

    const newAnomalyMap: AnomalyMap = {
      // Trace B has updated Anomaly for key 101 and a new anomaly for key 102
      traceB: {
        101: { ...dummyAnomaly, id: 1 },
        102: { ...dummyAnomaly, id: 2 },
      },
      // Introduce a new trace
      traceC: { 101: { ...dummyAnomaly, id: 2 } },
    };

    explore.addToAnomalyMap(newAnomalyMap);

    const expectedAnomalyMap: AnomalyMap = {
      traceA: { 101: { ...dummyAnomaly } },
      traceB: {
        101: { ...dummyAnomaly, id: 1 },
        102: { ...dummyAnomaly, id: 2 },
      },
      traceC: { 101: { ...dummyAnomaly, id: 2 } },
    };
    assert.deepEqual(explore.fullAnomalyMap, expectedAnomalyMap);
  });
});

describe('createGraphConfigs', () => {
  it('traceset without formulas', () => {
    const traceset = TraceSet({
      ',config=8888,arch=x86,': Trace([0.1, 0.2, 0.0, 0.4]),
      ',config=8888,arch=arm,': Trace([1.1, 1.2, 0.0, 1.4]),
      ',config=565,arch=x86,': Trace([0.0, 0.0, 3.3, 3.4]),
      ',config=565,arch=arm,': Trace([0.0, 0.0, 4.3, 4.4]),
    });
    const explore =
      setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();
    const result = explore.createGraphConfigs(traceset);
    const expected: GraphConfig[] = [
      {
        keys: '',
        formulas: [],
        queries: ['config=8888&arch=x86'],
      },
      {
        keys: '',
        formulas: [],
        queries: ['config=8888&arch=arm'],
      },
      {
        keys: '',
        formulas: [],
        queries: ['config=565&arch=x86'],
      },
      {
        keys: '',
        formulas: [],
        queries: ['config=565&arch=arm'],
      },
    ];

    assert.deepEqual(result, expected);
  });

  it('traceset with formulas', () => {
    const traceset = TraceSet({
      'func1(,config=8888,arch=x86,)': Trace([0.1, 0.2, 0.0, 0.4]),
      'func2(,config=8888,arch=arm,)': Trace([1.1, 1.2, 0.0, 1.4]),
    });
    const explore =
      setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();
    const result = explore.createGraphConfigs(traceset);
    const expected: GraphConfig[] = [
      {
        keys: '',
        formulas: ['func1(,config=8888,arch=x86,)'],
        queries: [],
      },
      {
        keys: '',
        formulas: ['func2(,config=8888,arch=arm,)'],
        queries: [],
      },
    ];

    assert.deepEqual(result, expected);
  });
});

describe('Default values', () => {
  beforeEach(() => {
    fetchMock.get('/_/login/status', {
      email: 'someone@example.org',
      roles: ['editor'],
    });
    fetchMock.post('/_/count/', {
      count: 117,
      paramset: {},
    });
    fetchMock.get(/_\/initpage\/.*/, () => ({
      dataframe: {
        traceset: null,
        header: null,
        paramset: {},
        skip: 0,
      },
      ticks: [],
      skps: [],
      msg: '',
    }));
  });
  it('Checks no default values', async () => {
    const defaultConfig: QueryConfig = {
      default_param_selections: null,
      default_url_values: null,
      include_params: null,
    };

    const defaultBody = JSON.stringify(defaultConfig);
    fetchMock.get('path:/_/defaults/', {
      status: 200,
      body: defaultBody,
    });

    const explore =
      await setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();
    await fetchMock.flush(true);
    const originalState = deepCopy(explore!.state);
    await explore['applyQueryDefaultsIfMissing']();

    const newState = explore.state;
    assert.deepEqual(newState, originalState);
  });

  it('Checks for default summary value', async () => {
    const defaultConfig: QueryConfig = {
      default_param_selections: null,
      default_url_values: {
        summary: 'true',
      },
      include_params: null,
    };

    const explore =
      setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();
    explore['_defaults'] = defaultConfig;

    const originalState = deepCopy(explore.state);
    await explore['applyQueryDefaultsIfMissing']();

    const newState = explore.state;
    assert.notDeepEqual(
      newState,
      originalState,
      'new state should not equal original state'
    );
    assert.isTrue(newState.summary);
  });
});

describe('requestFrameBodyDeltaFromState', () => {
  function fakeDataFrame(): DataFrame {
    const ret: DataFrame = {
      header: [
        { offset: CommitNumber(11), timestamp: TimestampSeconds(1100) },
        { offset: CommitNumber(12), timestamp: TimestampSeconds(1200) },
        { offset: CommitNumber(13), timestamp: TimestampSeconds(1300) },
        { offset: CommitNumber(14), timestamp: TimestampSeconds(1400) },
      ],
      traceset: TraceSet({
        ',config=8888,arch=x86,': Trace([0.1, 0.2, 0.0, 0.4]),
        ',config=8888,arch=arm,': Trace([1.1, 1.2, 0.0, 1.4]),
        ',config=565,arch=x86,': Trace([0.0, 0.0, 3.3, 3.4]),
        ',config=565,arch=arm,': Trace([0.0, 0.0, 4.3, 4.4]),
      }),
      paramset: ReadOnlyParamSet({}),
      skip: 0,
    };
    buildParamSet(ret);
    return ret;
  }

  afterEach(() => {
    fetchMock.reset();
  });

  it('fetches only missing older data when panning left with overlap', async () => {
    // dataframe:           [1100,   1400]
    //     state: [300,       1200]
    //   request: [300, 1100]

    const explore =
      await setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();

    // Setup is that we have the timestamp range [1100, 1400] loaded in the current dataframe.
    // User does something to cause this._state's timestamp range to become [300, 1200],
    // e.g. 'pan left'.
    const existingDataFrame = fakeDataFrame();
    explore['_dataframe'] = existingDataFrame;

    const state = deepCopy(explore.state);

    state._incremental = true;
    state.begin = 300;
    state.end = 1200;
    state.queries = fromParamSet(existingDataFrame.paramset).split('&');
    explore['_state'] = state;

    const frameBodyDeltaRequest: FrameRequest =
      explore['requestFrameBodyDeltaFromState']();

    // It should return a request for just the missing range, [300, 1100].
    assert.deepEqual(
      [frameBodyDeltaRequest.begin, frameBodyDeltaRequest.end],
      [300, 1100]
    );
    assert.includeMembers(frameBodyDeltaRequest.queries!, [
      'config=8888',
      'arch=x86',
      'config=565',
      'arch=arm',
    ]);
  });

  it('fetches entire range when _incremental is false ', async () => {
    // dataframe:       [1100,   1400]
    //     state: [300,          1400]
    //   request: [300,          1400]

    const explore =
      await setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();

    // Setup is that we have the timestamp range [1100, 1400] loaded in the current dataframe.
    // User does something to cause this._state's timestamp range to become [300, 1400],
    // e.g. 'pan left'.
    const existingDataFrame = fakeDataFrame();
    explore['_dataframe'] = existingDataFrame;

    const state = deepCopy(explore.state);

    state._incremental = false;
    state.begin = 300;
    state.end = 1400;
    explore['_state'] = state;

    fetchMock.post('/_/shift/', {
      begin: 0,
      end: 1200,
    });
    explore['zoomLeftKey']();
    assert.isTrue(fetchMock.done());

    const frameBodyDeltaRequest: FrameRequest =
      explore['requestFrameBodyDeltaFromState']();

    // It should return a request for just the missing range, [300, 1100].
    assert.deepEqual(
      [frameBodyDeltaRequest.begin, frameBodyDeltaRequest.end],
      [300, 1400]
    );
  });

  it('fetches only missing new data when panning right with overlap', async () => {
    // dataframe: [1100,   1400]
    //     state:   [1200,           2100]
    //   request:          [1400,    2100]

    const explore =
      await setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();

    // Setup is that we have the timestamp range [1100, 1400] loaded in the current dataframe.
    // User does something to cause this._state's timestamp range to become [1200, 2100],
    // e.g. 'pan right'.
    const existingDataFrame = fakeDataFrame();
    explore['_dataframe'] = existingDataFrame;

    const state = deepCopy(explore.state);

    state._incremental = true;
    state.begin = 1100;
    state.end = 2100;
    state.queries = fromParamSet(existingDataFrame.paramset).split('&');
    explore['_state'] = state;

    const frameBodyDeltaRequest: FrameRequest =
      explore['requestFrameBodyDeltaFromState']();

    // It should return a request for just the missing range, [1400, 2100].
    assert.deepEqual(
      [frameBodyDeltaRequest.begin, frameBodyDeltaRequest.end],
      [1400, 2100]
    );
  });

  it('fetches full range if zoomed out both left and right', async () => {
    // dataframe:    [1100,   1400]
    //     state: [700,              2100]
    //   request: [700,              2100]
    const explore =
      await setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();

    // Setup is that we have the timestamp range [1100, 1400] loaded in the current dataframe.
    // User does something to cause this._state's timestamp range to become [700, 2100],
    // e.g. 'zoom out past both edges'.
    const existingDataFrame = fakeDataFrame();
    explore['_dataframe'] = existingDataFrame;

    const state = deepCopy(explore.state);

    state._incremental = true;
    state.begin = 700;
    state.end = 2100;
    explore['_state'] = state;

    const frameBodyDeltaRequest: FrameRequest =
      explore['requestFrameBodyDeltaFromState']();

    // Even though incremental fetches are enabled, it should still return a
    // request for the full range, [700, 2100].
    assert.deepEqual(
      [frameBodyDeltaRequest.begin, frameBodyDeltaRequest.end],
      [700, 2100]
    );
  });

  it('fetches full range if state and dataframe ranges do not overlap', async () => {
    // dataframe:    [1100,   1400]
    //     state:                   [1900, 2100]
    //   request:                   [1900, 2100]
    const explore =
      await setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();

    // Setup is that we have the timestamp range [1100, 1400] loaded in the current dataframe.
    // User does something to cause this._state's timestamp range to become [700, 2100],
    // e.g. 'zoom out past both edges'.
    const existingDataFrame = fakeDataFrame();
    explore['_dataframe'] = existingDataFrame;

    const state = deepCopy(explore.state);

    state._incremental = true;
    state.begin = 1900;
    state.end = 2100;
    explore['_state'] = state;

    const frameBodyDeltaRequest: FrameRequest =
      explore['requestFrameBodyDeltaFromState']();

    // Even though incremental fetches are enabled, it should still return a
    // request for the full range, [700, 2100].
    assert.deepEqual(
      [frameBodyDeltaRequest.begin, frameBodyDeltaRequest.end],
      [1900, 2100]
    );
  });

  it('fetches full range when _incremental is false', async () => {
    // dataframe:       [1100,   1400]
    //     state: [300,          1400]
    //   request: [300,          1400]

    const explore =
      await setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();

    // Setup is that we have the timestamp range [1100, 1400] loaded in the current dataframe.
    // User does something to cause this._state's timestamp range to become [300, 1400],
    // e.g. 'pan left'.
    const existingDataFrame = fakeDataFrame();
    explore['_dataframe'] = existingDataFrame;

    const state = deepCopy(explore.state);

    state._incremental = false;
    state.begin = 300;
    state.end = 1400;
    explore['_state'] = state;

    const frameBodyDeltaRequest: FrameRequest =
      explore['requestFrameBodyDeltaFromState']();

    // It should return a request for the full _state.begin/end range, [300, 1400].
    assert.deepEqual(
      [frameBodyDeltaRequest.begin, frameBodyDeltaRequest.end],
      [300, 1400]
    );
  });

  it('handles pan and zoom requests when _incremental is false', async () => {
    // dataframe:       [1100,   1400]
    //     state: [900,        1200]
    //   request: [900,        1200]

    const explore =
      await setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();

    const existingDataFrame = fakeDataFrame();
    explore['_dataframe'] = existingDataFrame;
    let currentBounds = timestampBounds(existingDataFrame);
    const prePanZoom = explore['getCurrentZoom']();

    const state = deepCopy(explore.state);
    state.queries = ['name=IDK'];
    state._incremental = false;
    explore['_state'] = state;

    const shiftedDataFrame = deepCopy(existingDataFrame);
    shiftedDataFrame.header = [
      { offset: CommitNumber(9), timestamp: TimestampSeconds(900) },
      { offset: CommitNumber(10), timestamp: TimestampSeconds(1000) },
      { offset: CommitNumber(11), timestamp: TimestampSeconds(1100) },
      { offset: CommitNumber(12), timestamp: TimestampSeconds(1200) },
    ];
    const shiftResponse = {
      begin: shiftedDataFrame!.header[0]!.timestamp,
      end: shiftedDataFrame!.header[3]!.timestamp,
    };
    const finishedBody: progress.SerializedProgress = {
      status: 'Finished',
      messages: [],
      results: {
        anomalymap: null,
        skps: [],
        dataframe: shiftedDataFrame,
        display_mode: 'display_plot',
        msg: '',
      },
      url: '',
    };
    const startURL = '/_/frame/start';

    fetchMock.reset();
    fetchMock.post('/_/shift/', shiftResponse);
    fetchMock.post(startURL, finishedBody);

    // Simulate user hitting a key to pan left.
    explore['zoomLeftKey']();
    await fetchMock.flush(true);
    assert.isTrue(fetchMock.done(), 'made the expected fetch calls');

    const postPanZoom = explore['getCurrentZoom']();
    assert.deepEqual(
      prePanZoom,
      postPanZoom,
      'panning should not affect the zoom'
    );

    const newState = explore['_state'];
    assert.equal(newState.requestType, 0);
    const frameBodyDeltaRequest: FrameRequest =
      explore['requestFrameBodyDeltaFromState']();

    currentBounds = timestampBounds(explore['_dataframe']);
    // It should return a request for the full _state.begin/end range.
    assert.deepEqual(
      [frameBodyDeltaRequest.begin, frameBodyDeltaRequest.end],
      currentBounds,
      'frame req should match current bounds'
    );
    fetchMock.restore();
  });

  it('fetch full dataframe if queries change', async () => {
    // run a query, test=A, turn on _incremental,
    // then do another search with test=B and also change the date range on that second query.
    // dataframe (paramset: ['test=A']):       [1100,   1400]
    //      state (queries: ['test=B']): [900,        1200]
    //    request (queries: ['test=B']): [900,        1200]

    const explore =
      await setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();

    // run a query, test=A
    const queryTestADataFrame: DataFrame = {
      header: [
        { offset: CommitNumber(11), timestamp: TimestampSeconds(1100) },
        { offset: CommitNumber(12), timestamp: TimestampSeconds(1200) },
        { offset: CommitNumber(13), timestamp: TimestampSeconds(1300) },
        { offset: CommitNumber(14), timestamp: TimestampSeconds(1400) },
      ],
      traceset: TraceSet({
        'test=A': Trace([0.1, 0.2, 0.0, 0.4]),
      }),
      paramset: ReadOnlyParamSet({}),
      skip: 0,
    };
    buildParamSet(queryTestADataFrame);

    explore['_dataframe'] = queryTestADataFrame;
    const state = deepCopy(explore.state);

    // turn on _incremental
    state._incremental = true;
    // then do another search with test=B
    state.queries = ['test=B'];

    // and also change the date range on that second query
    state.begin = 900;
    state.end = 1200;

    explore['_state'] = state;

    const frameBodyDeltaRequest: FrameRequest =
      explore['requestFrameBodyDeltaFromState']();
    assert.deepEqual(
      frameBodyDeltaRequest.queries,
      ['test=B'],
      'fetch the new query rather than the existing query'
    );
    assert.deepEqual(
      [frameBodyDeltaRequest.begin, frameBodyDeltaRequest.end],
      [900, 1200],
      'fetch entire range if the query changed'
    );
  });
});

describe('plotSummary', () => {
  it('Populate Plot Summary bar', async () => {
    const explore =
      await setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();

    const state = deepCopy(explore.state);
    state.plotSummary = true;
    const queryTestADataFrame: DataFrame = {
      header: [
        { offset: CommitNumber(11), timestamp: TimestampSeconds(1100) },
        { offset: CommitNumber(12), timestamp: TimestampSeconds(1200) },
        { offset: CommitNumber(13), timestamp: TimestampSeconds(1300) },
        { offset: CommitNumber(14), timestamp: TimestampSeconds(1400) },
      ],
      traceset: TraceSet({
        'test=A': Trace([0.1, 0.2, 0.0, 0.4]),
      }),
      paramset: ReadOnlyParamSet({}),
      skip: 0,
    };
    buildParamSet(queryTestADataFrame);

    explore['fullDataFrame'] = queryTestADataFrame;
    explore['populatePlotSummary']();
    const plotSummaryElement = explore['plotSummary'];
    assert.isNotNull(plotSummaryElement);
    assert.isNotNull(plotSummaryElement!['currentChartData']);
  });

  it('Plot Summary bar not enabled', async () => {
    const explore =
      await setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();

    explore['fullDataFrame'] = null;
    explore['populatePlotSummary']();
    const plotSummaryElement = explore['plotSummary'];
    assert.isNotNull(plotSummaryElement);
    assert.isNull(plotSummaryElement!['currentChartData']);
  });

  it('Populate Extended timeline', async () => {
    const finishedBody: progress.SerializedProgress = {
      status: 'Finished',
      messages: [],
      results: {},
      url: '',
    };
    fetchMock.post('/_/frame/start', finishedBody);
    const defaultConfig: QueryConfig = {
      default_param_selections: null,
      default_url_values: null,
      include_params: null,
    };

    const defaultBody = JSON.stringify(defaultConfig);
    fetchMock.get('path:/_/defaults/', {
      status: 200,
      body: defaultBody,
    });

    fetchMock.post('/_/count/', {
      count: 0,
      paramset: {},
    });

    fetchMock.get(/_\/initpage\/.*/, () => ({
      dataframe: {
        traceset: null,
        header: null,
        paramset: {},
        skip: 0,
      },
      ticks: [],
      skps: [],
      msg: '',
    }));

    const explore =
      await setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();
    const queryTestADataFrame: DataFrame = {
      header: [
        { offset: CommitNumber(11), timestamp: TimestampSeconds(1100) },
        { offset: CommitNumber(12), timestamp: TimestampSeconds(1200) },
        { offset: CommitNumber(13), timestamp: TimestampSeconds(1300) },
        { offset: CommitNumber(14), timestamp: TimestampSeconds(1400) },
      ],
      traceset: TraceSet({
        'test=A': Trace([0.1, 0.2, 0.0, 0.4]),
      }),
      paramset: ReadOnlyParamSet({}),
      skip: 0,
    };
    buildParamSet(queryTestADataFrame);

    explore['fullDataFrame'] = queryTestADataFrame;
    explore.state.plotSummary = true;
    explore['populateExtendedTimelineForPlotSummary']();
    await fetchMock.flush(true);
    assert.isTrue(fetchMock.done('/_/frame/start'));
  });
});
