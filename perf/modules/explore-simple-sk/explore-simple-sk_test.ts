/* eslint-disable dot-notation */
import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import {
  ColumnHeader,
  CommitNumber,
  FrameResponse,
  QueryConfig,
  TimestampSeconds,
  Trace,
  TraceSet,
  DataFrame,
  ReadOnlyParamSet,
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
  State,
} from './explore-simple-sk';
import { MdDialog } from '@material/web/dialog/dialog';
import { MdSwitch } from '@material/web/switch/switch';
import { PlotSummarySk } from '../plot-summary-sk/plot-summary-sk';
import { setUpElementUnderTest, waitForRender } from '../../../infra-sk/modules/test_util';
import { generateFullDataFrame } from '../dataframe/test_utils';
import sinon from 'sinon';
// Import for side effects. Make `plotSummary`(has no direct interaction with the module)
// work when run in isolation.
import './explore-simple-sk';

fetchMock.config.overwriteRoutes = true;

const now = 1726081856; // an arbitrary UNIX time;
const timeSpan = 89; // an arbitrary prime number for time span between commits .

window.perf = {
  instance_url: '',
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
  keys_for_useful_links: [],
  skip_commit_detail_display: false,
  image_tag: 'fake-tag',
  remove_default_stat_value: false,
  enable_skia_bridge_aggregation: false,
  show_json_file_display: false,
  always_show_commit_info: false,
  show_triage_link: true,
  show_bisect_btn: true,
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

describe('createGraphConfigs', () => {
  it('traceset without formulas', () => {
    const traceset = TraceSet({
      ',config=8888,arch=x86,': Trace([0.1, 0.2, 0.0, 0.4]),
      ',config=8888,arch=arm,': Trace([1.1, 1.2, 0.0, 1.4]),
      ',config=565,arch=x86,': Trace([0.0, 0.0, 3.3, 3.4]),
      ',config=565,arch=arm,': Trace([0.0, 0.0, 4.3, 4.4]),
    });
    const explore = setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();
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
    const explore = setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();
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

    const explore = await setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();
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

    const explore = setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();
    explore['_defaults'] = defaultConfig;

    const originalState = deepCopy(explore.state);
    await explore['applyQueryDefaultsIfMissing']();

    const newState = explore.state;
    assert.notDeepEqual(newState, originalState, 'new state should not equal original state');
    assert.isTrue(newState.summary);
  });
});

describe('plotSummary', () => {
  it('Populate Plot Summary bar', async () => {
    const explore = setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();

    explore.state.plotSummary = true;
    explore['tracesRendered'] = true;
    explore.render();

    const plotSummaryElement = explore['plotSummary'].value;
    assert.notEqual(plotSummaryElement, undefined);
  });

  it('Plot Summary bar not enabled', async () => {
    const explore = setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();
    explore.render();

    const plotSummaryElement = explore['plotSummary'].value;
    assert.equal(plotSummaryElement, undefined);
  });
});

describe('updateBrowserURL', () => {
  let explore: ExploreSimpleSk;

  beforeEach(() => {
    explore = setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();
  });

  afterEach(() => {
    fetchMock.reset(); // Reset fetch mocks
  });

  it('should add begin, end, and request_type=0 params when none exist', () => {
    explore.state.begin = 100;
    explore.state.end = 200;
    explore['updateBrowserURL']();
    const pushedUrl = new URL(window.location.href as string);
    assert.equal(pushedUrl.searchParams.get('begin'), '100');
    assert.equal(pushedUrl.searchParams.get('end'), '200');
    assert.equal(pushedUrl.searchParams.get('request_type'), '0');
  });
});

describe('rationalizeTimeRange', () => {
  let explore: ExploreSimpleSk;
  let clock: sinon.SinonFakeTimers;
  const now = 1672531200; // Jan 1, 2023 in seconds

  beforeEach(() => {
    explore = setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();
    clock = sinon.useFakeTimers(now * 1000);
  });

  afterEach(() => {
    clock.restore();
  });

  it('handles uninitialized begin and end', () => {
    const state = new State();
    const rationalizedState = explore['rationalizeTimeRange'](state);
    assert.equal(rationalizedState.end, now);
    assert.closeTo(rationalizedState.begin, now - 24 * 60 * 60, 1);
  });

  it('corrects inverted time ranges', () => {
    const state = new State();
    state.begin = now - 100;
    state.end = now - 500;
    const rationalizedState = explore['rationalizeTimeRange'](state);
    assert.isTrue(rationalizedState.end > rationalizedState.begin);
  });

  it('handles zero-length time ranges', () => {
    const state = new State();
    state.begin = now - 100;
    state.end = now - 100;
    const rationalizedState = explore['rationalizeTimeRange'](state);
    assert.isTrue(rationalizedState.end > rationalizedState.begin);
  });

  it('ensures end is not in the future', () => {
    const state = new State();
    state.begin = now - 100;
    state.end = now + 500;
    const rationalizedState = explore['rationalizeTimeRange'](state);
    assert.equal(rationalizedState.end, now);
  });
});

describe('updateTestPickerUrl', () => {
  let explore: ExploreSimpleSk;

  beforeEach(async () => {
    // Mock all fetches that can be triggered by element creation.
    fetchMock.get(/.*\/_\/initpage\/.*/, {
      dataframe: { paramset: {} },
    });
    fetchMock.get('/_/login/status', {
      email: 'someone@example.org',
      roles: ['editor'],
    });

    explore = setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();
    await fetchMock.flush(true);
  });

  afterEach(() => {
    fetchMock.reset();
  });

  it('should set the URL to "#" when there are no queries, formulas, or keys', (done) => {
    explore.state.queries = [];
    explore.state.formulas = [];
    explore.state.keys = '';
    explore['updateTestPickerUrl']();
    setTimeout(() => {
      assert.equal(explore['testPickerUrl'], '#');
      done();
    });
  });

  it('should construct the correct URL when there are queries', (done) => {
    fetchMock.post('/_/shortcut/update', { id: 'shortcut123' });
    explore.state.queries = ['config=test'];
    explore.state.formulas = [];
    explore.state.keys = '';
    explore.state.begin = 123;
    explore.state.end = 456;
    explore.state.requestType = 0;

    explore['updateTestPickerUrl']();
    setTimeout(() => {
      assert.equal(
        explore['testPickerUrl'],
        '/m/?begin=123&end=456&request_type=0&shortcut=shortcut123&totalGraphs=1'
      );
      done();
    });
  });
});

describe('Incremental Trace Loading', () => {
  let explore: ExploreSimpleSk;

  // Define clean queries and comma-wrapped keys separately.
  const initialQuery = 'arch=x86&config=original';
  const initialTraceKey = `,${initialQuery},`; // Comma-wrapped for traceset
  const newQuery = 'arch=arm&config=new';
  const newTraceKey = `,${newQuery},`; // Comma-wrapped for traceset

  // Generate a dataframe and ensure it has traceMetadata.
  const initialDataFrame = generateFullDataFrame(
    { begin: 1, end: 100 },
    now,
    1,
    [timeSpan],
    [[10, 20]],
    [initialTraceKey]
  );
  initialDataFrame.traceMetadata = []; // Ensure traceMetadata exists.

  const initialFrameResponse: FrameResponse = {
    dataframe: initialDataFrame,
    anomalymap: {},
    skps: [],
    msg: '',
    display_mode: 'display_plot',
  };

  // Generate the second dataframe and ensure it has traceMetadata.
  const newDataFrame = generateFullDataFrame(
    { begin: 1, end: 100 },
    now,
    1,
    [timeSpan],
    [[30, 40]],
    [newTraceKey]
  );
  newDataFrame.traceMetadata = []; // Ensure traceMetadata exists.

  const newFrameResponse: FrameResponse = {
    dataframe: newDataFrame,
    anomalymap: {},
    skps: [],
    msg: '',
    display_mode: 'display_plot',
  };

  beforeEach(() => {
    fetchMock.reset();
    explore = setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();

    fetchMock.get('/_/login/status', {
      email: 'someone@example.org',
      roles: ['editor'],
    });
    fetchMock.get('path:/_/defaults/', {
      status: 200,
      body: JSON.stringify({}),
    });
    fetchMock.post('/_/frame/start', {});
    fetchMock.post('/_/user_issues/', { UserIssues: [] });
    fetchMock.flush(true);
  });

  it('only fetches new data when adding a trace', async () => {
    // Original data.
    fetchMock.postOnce('/_/frame/start', (_url, opts) => {
      const body = JSON.parse(opts.body as string);
      assert.deepEqual(body.queries, [initialQuery]);
      return {
        status: 'Finished',
        results: initialFrameResponse,
        messages: [],
      };
    });
    await explore.addFromQueryOrFormula(true, 'query', initialQuery, '');
    await fetchMock.flush(true);

    // Verify the initial data is present.
    assert.containsAllKeys(explore['_dataframe'].traceset, [initialTraceKey]);
    assert.lengthOf(Object.keys(explore['_dataframe'].traceset), 1);

    // New data, expect incremental fetch.
    fetchMock.postOnce('/_/frame/start', (_url, opts) => {
      const body = JSON.parse(opts.body as string);
      assert.deepEqual(body.queries, [newQuery]);
      return {
        status: 'Finished',
        results: newFrameResponse,
        messages: [],
      };
    });

    await explore.addFromQueryOrFormula(false, 'query', newQuery, '');
    await fetchMock.flush(true);

    const finalTraceset = explore['_dataframe'].traceset;
    assert.containsAllKeys(finalTraceset, [initialTraceKey, newTraceKey]);
    assert.lengthOf(Object.keys(finalTraceset), 2);
  });
});

describe('State Management', () => {
  let explore: ExploreSimpleSk;
  let updateTestPickerUrlStub: sinon.SinonStub;

  beforeEach(async () => {
    // Mock all fetches that can be triggered by element creation.
    fetchMock.get(/.*\/_\/initpage\/.*/, {
      dataframe: { paramset: {} },
    });
    fetchMock.get('/_/login/status', {
      email: 'someone@example.org',
      roles: ['editor'],
    });
    fetchMock.post('/_/frame/start', {
      status: 'Finished',
      results: { dataframe: { traceset: {} } },
    });

    explore = setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();
    await fetchMock.flush(true);

    // Stub the method that gets called when a state change is detected.
    updateTestPickerUrlStub = sinon.stub(explore, 'updateTestPickerUrl' as any);
  });

  afterEach(() => {
    fetchMock.reset();
    sinon.restore();
  });

  it('detects query changes with special characters due to encoding', () => {
    const state1 = new State();
    state1.queries = ['config=a b'];
    explore.state = state1;

    // Reset the stub after the initial state setting.
    updateTestPickerUrlStub.resetHistory();

    const state2 = new State();
    state2.queries = ['config=a+b'];
    explore.state = state2;

    // 'a b' and 'a+b' are different strings, so a change should be detected.
    assert.isTrue(
      updateTestPickerUrlStub.called,
      "URL update should be called for different queries ('a b' vs 'a+b')"
    );

    updateTestPickerUrlStub.resetHistory();

    const state3 = new State();
    state3.queries = ['config=a+b'];
    explore.state = state3;

    // The queries are identical, so no change should be detected.
    assert.isFalse(
      updateTestPickerUrlStub.called,
      'URL update should not be called for identical queries'
    );
  });
});

describe('x-axis domain switching', () => {
  const INITIAL_TIMESTAMP_BEGIN = 1672531200;
  const INITIAL_TIMESTAMP_END = 1672542000;
  const COMMIT_101 = 101;
  const COMMIT_102 = 102;
  const TIMESTAMP_101 = 1672534800;
  const TIMESTAMP_102 = 1672538400;
  const ROUNDING_TOLERANCE_SECONDS = 120;
  // A simple header for converting between commit offsets and timestamps.
  const testHeader: ColumnHeader[] = [
    {
      offset: CommitNumber(100),
      timestamp: TimestampSeconds(1672531200),
      hash: 'h1',
      author: '',
      message: '',
      url: '',
    },
    {
      offset: CommitNumber(101),
      timestamp: TimestampSeconds(1672534800),
      hash: 'h2',
      author: '',
      message: '',
      url: '',
    },
    {
      offset: CommitNumber(102),
      timestamp: TimestampSeconds(1672538400),
      hash: 'h3',
      author: '',
      message: '',
      url: '',
    },
    {
      offset: CommitNumber(103),
      timestamp: TimestampSeconds(1672542000),
      hash: 'h4',
      author: '',
      message: '',
      url: '',
    },
  ];

  const testDataFrame: DataFrame = {
    traceset: TraceSet({
      ',config=test,': Trace([1, 2, 3, 4]),
    }),
    header: testHeader,
    paramset: {} as ReadOnlyParamSet,
    skip: 0,
    traceMetadata: [],
  };

  const testFrameResponse: FrameResponse = {
    dataframe: testDataFrame,
    anomalymap: null,
    display_mode: 'display_plot',
    skps: [],
    msg: '',
  };

  let explore: ExploreSimpleSk;

  beforeEach(async () => {
    explore = setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();
    await window.customElements.whenDefined('explore-simple-sk');
    await window.customElements.whenDefined('dataframe-repository-sk');
    await window.customElements.whenDefined('plot-summary-sk');
  });

  // Helper function to set up the component for domain switching tests.
  async function setupDomainSwitchTest(domain: 'date' | 'commit'): Promise<PlotSummarySk> {
    explore.state = {
      ...explore.state,
      queries: ['config=test'],
      domain: domain,
      plotSummary: true,
      begin: INITIAL_TIMESTAMP_BEGIN,
      end: INITIAL_TIMESTAMP_END,
      requestType: 0,
    };
    await waitForRender(explore);

    // Provide data to the component.
    await explore.UpdateWithFrameResponse(
      testFrameResponse,
      {
        begin: explore.state.begin,
        end: explore.state.end,
        num_commits: 250,
        request_type: 0,
        formulas: [],
        queries: ['config=test'],
        keys: '',
        tz: 'UTC',
        pivot: null,
        disable_filter_parent_traces: false,
      },
      false,
      null,
      false
    );
    await waitForRender(explore);

    await new Promise((resolve) => setTimeout(resolve, 0));

    const plotSummary = explore.querySelector('plot-summary-sk') as PlotSummarySk;
    assert.exists(plotSummary, 'The plot-summary-sk element should be in the DOM.');
    return plotSummary;
  }

  it('preserves selection when switching from date to commit', async () => {
    const plotSummary = await setupDomainSwitchTest('date');

    // Set an initial time-based selection on plot-summary-sk
    const initialSelection = { begin: TIMESTAMP_101, end: TIMESTAMP_102 };
    plotSummary.selectedValueRange = initialSelection;
    await plotSummary.updateComplete;
    await new Promise((resolve) => setTimeout(resolve, 0));

    const settingsDialog = explore.querySelector('#settings-dialog') as MdDialog;
    const switchEl = settingsDialog.querySelector('#commit-switch') as MdSwitch;
    assert.exists(switchEl, '#commit-switch element not found.');

    // Simulate switching to 'commit' domain (selected = false)
    switchEl!.selected = false;
    switchEl!.dispatchEvent(new Event('change'));

    // Wait for ExploreSimpleSk to handle the change and update its children
    await waitForRender(explore);
    await plotSummary.updateComplete;
    await new Promise((resolve) => setTimeout(resolve, 100));

    // rare, but can be flaky here. Since it wait for async event.
    // Increase of timeout above can help.
    assert.exists(
      plotSummary.selectedValueRange,
      'selectedValueRange should not be null after switch'
    );

    // Although the actual 'begin' and 'end' values are integers, they can be converted to
    // floating-point numbers for UI rendering to prevent the graph from "jumping" when the x-axis
    // domain is switched. The approximation in this test is used solely to prevent failures caused
    // by floating-point arithmetic inaccuracies, e.g., 101 !== 101.000000001.
    assert.approximately(
      plotSummary.selectedValueRange.begin as number,
      COMMIT_101,
      1e-3,
      'Selected range.begin did not convert correctly'
    );

    assert.approximately(
      plotSummary.selectedValueRange.end as number,
      COMMIT_102,
      1e-3,
      'Selected range.end did not convert correctly'
    );

    assert.equal(explore.state.domain, 'commit', 'Explore state domain should be commit');
    assert.equal(plotSummary.domain, 'commit', 'PlotSummary domain property should be commit');
  });

  it('preserves selection when switching from commit to date', async () => {
    const roundingToleranceSeconds = ROUNDING_TOLERANCE_SECONDS;
    const plotSummary = await setupDomainSwitchTest('commit');

    // Set an initial commit-based selection on plot-summary-sk
    const initialSelection = { begin: COMMIT_101, end: COMMIT_102 };
    plotSummary.selectedValueRange = initialSelection;
    await plotSummary.updateComplete;
    await new Promise((resolve) => setTimeout(resolve, 0));

    const settingsDialog = explore.querySelector('#settings-dialog') as MdDialog;
    const switchEl = settingsDialog.querySelector('#commit-switch') as MdSwitch;
    assert.exists(switchEl, '#commit-switch element not found.');

    // Simulate switching to 'date' domain (selected = true)
    switchEl!.selected = true;
    switchEl!.dispatchEvent(new Event('change'));

    await waitForRender(explore);
    await plotSummary.updateComplete;
    await new Promise((resolve) => setTimeout(resolve, 100));

    assert.exists(
      plotSummary.selectedValueRange,
      'selectedValueRange should not be null after switch'
    );

    assert.approximately(
      plotSummary.selectedValueRange.begin as number,
      TIMESTAMP_101,
      roundingToleranceSeconds,
      'Selected range.begin did not convert correctly'
    );

    assert.approximately(
      plotSummary.selectedValueRange.end as number,
      TIMESTAMP_102,
      roundingToleranceSeconds,
      'Selected range.end did not convert correctly'
    );

    assert.equal(explore.state.domain, 'date', 'Explore state domain should be date');
    assert.equal(plotSummary.domain, 'date', 'PlotSummary domain property should be date');
  });
});
