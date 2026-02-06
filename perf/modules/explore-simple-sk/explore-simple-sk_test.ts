/* eslint-disable dot-notation */
import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import {
  ColumnHeader,
  CommitNumber,
  FrameResponse,
  FrameRequest,
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
  State,
} from './explore-simple-sk';
import { GraphConfig, updateShortcut } from '../common/graph-config';
import { MdDialog } from '@material/web/dialog/dialog';
import { MdSwitch } from '@material/web/switch/switch';
import { PlotSummarySk } from '../plot-summary-sk/plot-summary-sk';
import { setUpElementUnderTest, waitForRender } from '../../../infra-sk/modules/test_util';
import { generateFullDataFrame } from '../dataframe/test_utils';
import sinon from 'sinon';
// Import for side effects. Make `plotSummary`(has no direct interaction with the module)
// work when run in isolation.
import './explore-simple-sk';
import { DataService } from '../data-service';

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
  instance_name: 'chrome-perf-test',
  header_image_url: '',
  enable_v2_ui: false,
  dev_mode: false,
  extra_links: null,
};

describe('calculateRangeChange', () => {
  const offsets: CommitRange = [100, 120] as CommitRange;

  it('finds a left range increase', () => {
    const zoom: CommitRange = [-1, 10] as CommitRange;
    const clampedZoom: CommitRange = [0, 10] as CommitRange;

    const ret = calculateRangeChange(zoom, clampedZoom, offsets);
    assert.isTrue(ret.rangeChange);

    // We shift left by RANGE_CHANGE_ON_ZOOM_PERCENT of the total range.
    assert.deepEqual(ret.newOffsets, [90, 110]);
  });

  it('finds a right range increase', () => {
    const zoom: CommitRange = [0, 12] as CommitRange;
    const clampedZoom: CommitRange = [0, 10] as CommitRange;

    const ret = calculateRangeChange(zoom, clampedZoom, offsets);
    assert.isTrue(ret.rangeChange);

    // We shift right by RANGE_CHANGE_ON_ZOOM_PERCENT of the total range.
    assert.deepEqual(ret.newOffsets, [110, 130]);
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

describe('zoomKey', () => {
  let element: ExploreSimpleSk;

  beforeEach(async () => {
    // We need to flush the fetch mock to ensure the element is initialized
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
    element = setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();
    await fetchMock.flush(true);

    (element as any).plotSummary = {
      value: {
        selectedValueRange: { begin: 100, end: 200 },
      },
    } as any;
    (element as any).dfRepo = {
      value: {
        dataframe: {
          header: [
            { offset: 0, timestamp: 1000 },
            { offset: 300, timestamp: 2000 },
          ],
        },
      } as any,
    } as any;
    // Mock summarySelected to verify it's called
    sinon.stub(element, 'summarySelected');
    element.state.domain = 'commit';
  });

  afterEach(() => {
    sinon.restore();
  });

  it('zooms in', () => {
    // Zoom in by 10% of 100 (range 100-200) = 10.
    // New range should be [110, 190].
    element.onZoomIn();

    assert.isTrue((element.summarySelected as sinon.SinonStub).calledOnce);
    const event = (element.summarySelected as sinon.SinonStub).firstCall.args[0] as CustomEvent;
    assert.deepEqual(event.detail.value, { begin: 110, end: 190 });
    assert.deepEqual((element as any).plotSummary.value.selectedValueRange, {
      begin: 110,
      end: 190,
    });
  });

  it('zooms out', () => {
    // Zoom out by 10% of 100 (range 100-200) = 10.
    // New range should be [90, 210].
    element.onZoomOut();

    assert.isTrue((element.summarySelected as sinon.SinonStub).calledOnce);
    const event = (element.summarySelected as sinon.SinonStub).firstCall.args[0] as CustomEvent;
    assert.deepEqual(event.detail.value, { begin: 90, end: 210 });
    assert.deepEqual((element as any).plotSummary.value.selectedValueRange, {
      begin: 90,
      end: 210,
    });
  });

  it('pans left', () => {
    // Pan left by 10% of 100 = 10.
    // New range should be [90, 190].
    element.onPanLeft();

    assert.isTrue((element.summarySelected as sinon.SinonStub).calledOnce);
    const event = (element.summarySelected as sinon.SinonStub).firstCall.args[0] as CustomEvent;
    assert.deepEqual(event.detail.value, { begin: 90, end: 190 });
    assert.deepEqual((element as any).plotSummary.value.selectedValueRange, {
      begin: 90,
      end: 190,
    });
  });

  it('pans right', () => {
    // Pan right by 10% of 100 = 10.
    // New range should be [110, 210].
    element.onPanRight();

    assert.isTrue((element.summarySelected as sinon.SinonStub).calledOnce);
    const event = (element.summarySelected as sinon.SinonStub).firstCall.args[0] as CustomEvent;
    assert.deepEqual(event.detail.value, { begin: 110, end: 210 });
    assert.deepEqual((element as any).plotSummary.value.selectedValueRange, {
      begin: 110,
      end: 210,
    });
  });

  it('triggers fetch when zooming out of bounds', () => {
    // Mock zoomOrRangeChange to verify it's called fallback
    const zoomOrRangeChangeStub = sinon.stub(element as any, 'zoomOrRangeChange');
    fetchMock.post('/_/shift/', {
      begin: 0,
      end: 300,
    });

    // Set range close to edge [0, 100] with data [0, 300]
    (element as any).plotSummary.value.selectedValueRange = { begin: 0, end: 100 };

    // Zoom out will try to go to [-5, 105]. -5 is out of bounds [0, 300].
    // Should trigger zoomOrRangeChange (fallback path).
    element.onZoomOut();

    assert.isTrue(zoomOrRangeChangeStub.calledOnce);
    assert.isFalse((element.summarySelected as sinon.SinonStub).called);
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

  describe('JSON Input', () => {
    let explore: ExploreSimpleSk;

    beforeEach(async () => {
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
    });

    afterEach(() => {
      fetchMock.reset();
    });

    it('parses valid JSON and updates state', async () => {
      const json = JSON.stringify({
        graphs: [
          {
            queries: ['config=test'],
            formulas: ['formula1'],
            keys: 'key1',
          },
        ],
      });

      await explore.addFromQueryOrFormula(true, 'json', '', '', json);

      assert.deepEqual(explore.state.queries, ['config=test']);
      assert.deepEqual(explore.state.formulas, ['formula1']);
      assert.equal(explore.state.keys, 'key1');
    });

    it('handles empty JSON gracefully', async () => {
      await explore.addFromQueryOrFormula(true, 'json', '', '', '');
      // Should not update state if JSON is empty (errorMessage is called, but state remains)
    });

    it('handles invalid JSON gracefully', async () => {
      await explore.addFromQueryOrFormula(true, 'json', '', '', '{invalid');
      // Should catch error and return.
    });

    it('reads JSON from URL param', async () => {
      const json = JSON.stringify({
        graphs: [
          {
            queries: ['config=url'],
          },
        ],
      });
      const url = new URL(window.location.href);
      url.searchParams.set('json', json);
      window.history.pushState({}, 'Test', url.toString());

      const spy = sinon.spy(explore, 'addFromQueryOrFormula');
      explore.useBrowserURL();

      assert.isTrue(spy.calledWith(true, 'json'));
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

    it('correctly syncs state to the domain picker on connectedCallback', async () => {
      // Create a new element to test connectedCallback in isolation.
      const testElement = document.createElement('explore-simple-sk') as ExploreSimpleSk;

      // Set an initial state before the element is connected.
      const initialState = new State();
      initialState.begin = 1000;
      initialState.end = 2000;
      initialState.numCommits = 150;
      initialState.requestType = 1;
      testElement.state = initialState;

      // The range picker should not exist yet.
      assert.isNull(testElement.querySelector('domain-picker-sk'));

      // Append to the DOM to trigger connectedCallback.
      document.body.appendChild(testElement);
      await waitForRender(testElement);

      // Now, the range picker should exist and its state should be synced.
      const rangePicker = testElement.querySelector('domain-picker-sk') as any;
      assert.isNotNull(rangePicker);
      assert.deepEqual(rangePicker.state, {
        begin: 1000,
        end: 2000,
        num_commits: 150,
        request_type: 1,
      });

      // Cleanup
      document.body.removeChild(testElement);
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
      fetchMock.get(/.*\/_\/initpage\/.*/, {
        dataframe: { paramset: {} },
      });
      fetchMock.get('/_/login/status', {
        email: 'someone@example.org',
        roles: ['editor'],
      });
      fetchMock.post('/_/count/', {
        count: 117,
        paramset: {},
      });
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
      // Wait for ExploreSimpleSk to handle the change and update its children
      await waitForRender(explore);
      await plotSummary.updateComplete;
      await new Promise((resolve) => setTimeout(resolve, 500));

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

      // Wait for ExploreSimpleSk to handle the change and update its children
      await waitForRender(explore);
      await plotSummary.updateComplete;
      await new Promise((resolve) => setTimeout(resolve, 500));

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

  describe('reset', () => {
    let explore: ExploreSimpleSk;

    beforeEach(async () => {
      explore = setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();
      await fetchMock.flush(true);
      explore.state.queries = ['a=b'];
      explore.state.formulas = ['norm()'];
      explore.state.keys = 'somekeys';
    });

    afterEach(() => {
      sinon.restore();
    });

    it('should call removeAll and queryDialog.show when use_test_picker_query is false', async () => {
      explore.openQueryByDefault = true;
      explore.reset();
      await waitForRender(explore);
      assert.isTrue(explore['_dialogOn']);
      assert.isEmpty(explore.state.queries);
      assert.isEmpty(explore.state.formulas);
      assert.isEmpty(explore.state.keys);
    });

    it('should only call removeAll when use_test_picker_query is true', async () => {
      explore.openQueryByDefault = false;
      explore.reset();
      await waitForRender(explore);
      assert.isFalse(explore['_dialogOn']);
      assert.isEmpty(explore.state.queries);
      assert.isEmpty(explore.state.formulas);
      assert.isEmpty(explore.state.keys);
    });
  });

  describe('JSON Input', () => {
    let explore: ExploreSimpleSk;

    beforeEach(async () => {
      fetchMock.get(/.*\/_\/initpage\/.*/, {
        dataframe: { paramset: {} },
      });
      fetchMock.get('/_/login/status', {
        email: 'someone@example.org',
        roles: ['editor'],
      });
      fetchMock.post('/_/frame/start', {
        status: 'Finished',
        results: {
          dataframe: {
            traceset: {},
            header: [
              { offset: 100, timestamp: 1000 },
              { offset: 101, timestamp: 1001 },
            ],
            paramset: {},
          },
          skps: [],
          msg: '',
        },
        messages: [],
      });
      fetchMock.post('/_/shortcut/update', { id: '123' });

      explore = setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();
      await fetchMock.flush(true);
    });

    afterEach(() => {
      fetchMock.reset();
    });

    it('parses valid JSON and updates state', async () => {
      const json = JSON.stringify({
        graphs: [
          {
            queries: ['config=test'],
            formulas: ['formula1'],
            keys: 'key1',
          },
        ],
      });

      await explore.addFromQueryOrFormula(true, 'json', '', '', json);

      assert.deepEqual(explore.state.queries, ['config=test']);
      assert.deepEqual(explore.state.formulas, ['formula1']);
      assert.equal(explore.state.keys, 'key1');
    });

    it('handles empty JSON gracefully', async () => {
      await explore.addFromQueryOrFormula(true, 'json', '', '', '');
    });

    it('handles invalid JSON gracefully', async () => {
      await explore.addFromQueryOrFormula(true, 'json', '', '', '{invalid');
    });

    describe('JSON Input', () => {
      let pushStateStub: sinon.SinonStub;

      beforeEach(() => {
        const desc = Object.getOwnPropertyDescriptor(window.history, 'pushState');
        console.log('DEBUG: pushState descriptor', desc);
        pushStateStub = sinon.stub(window.history, 'pushState');
      });

      afterEach(() => {
        pushStateStub.restore();
      });

      it('reads JSON from URL param', async () => {
        const json = JSON.stringify({
          graphs: [
            {
              queries: ['config=url'],
              formulas: [],
              keys: '',
            },
          ],
        });
        const url = new URL(window.location.href);
        url.searchParams.set('json', json);

        const stub = sinon.stub(explore, 'addFromQueryOrFormula').callsFake(() => {
          return Promise.resolve();
        });
        explore.useBrowserURL(true, url);

        assert.isTrue(stub.calledWith(true, 'json'));
      });
    });
  });
});

describe('Keyboard Shortcuts', () => {
  let explore: ExploreSimpleSk;

  beforeEach(async () => {
    explore = setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();
    fetchMock.post('/_/fe_telemetry', 200);
    await fetchMock.flush(true);
  });

  it('triggers triage actions on key press', async () => {
    // Mock tooltip and its methods
    const tooltip = explore.querySelector('chart-tooltip-sk') as any;
    tooltip.openNewBug = sinon.spy();
    tooltip.openExistingBug = sinon.spy();
    tooltip.ignoreAnomaly = sinon.spy();

    // Mock anomaly presence
    const anomaly = {
      id: '123',
      bug_id: 0,
      test_path: 'master/bot/benchmark/test',
      start_revision: 123,
      end_revision: 125,
      is_improvement: false,
      recovered: false,
      state: 'regression',
      statistic: 'avg',
      units: 'ms',
      degrees_of_freedom: 1,
      median_before_anomaly: 10,
      median_after_anomaly: 20,
      p_value: 0.01,
      segment_size_after: 10,
      segment_size_before: 10,
      std_dev_before_anomaly: 1,
      t_statistic: 5,
      subscription_name: 'sub',
      bug_component: '',
      bug_labels: [],
      bug_cc_emails: [],
      bisect_ids: [],
    };
    tooltip.anomaly = anomaly;
    (explore as any).selectedAnomaly = anomaly;

    // Trigger 'p' key for New Bug
    explore.keyDown(new KeyboardEvent('keydown', { key: 'p' }));
    assert.isTrue(tooltip.openNewBug.calledOnce, 'p key should trigger openNewBug');

    // Trigger 'n' key for Ignore
    explore.keyDown(new KeyboardEvent('keydown', { key: 'n' }));
    assert.isTrue(tooltip.ignoreAnomaly.calledOnce, 'n key should trigger ignoreAnomaly');

    // Trigger 'e' key for Existing Bug
    explore.keyDown(new KeyboardEvent('keydown', { key: 'e' }));
    assert.isTrue(tooltip.openExistingBug.calledOnce, 'e key should trigger openExistingBug');
  });
});

describe('Even X-Axis Spacing toggle', () => {
  let explore: ExploreSimpleSk;
  let switchEl: MdSwitch;
  let eventSpy: sinon.SinonSpy;

  beforeEach(async () => {
    fetchMock.get(/.*\/_\/initpage\/.*/, {
      dataframe: { paramset: {} },
    });
    fetchMock.get('/_/login/status', {
      email: 'someone@example.org',
      roles: ['editor'],
    });
    explore = setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();
    await waitForRender(explore);
    const settingsDialog = explore.querySelector('#settings-dialog') as MdDialog;
    switchEl = settingsDialog.querySelector('#even-x-axis-spacing-switch') as MdSwitch;
    eventSpy = sinon.spy();
    explore.addEventListener('even-x-axis-spacing-changed', eventSpy);
  });

  afterEach(() => {});

  it('should have the switch element', () => {
    assert.exists(switchEl);
  });

  it('should be unchecked by default', () => {
    assert.isFalse(switchEl.selected);
    assert.isFalse(explore.state.evenXAxisSpacing);
  });

  it('should update state and fire event when toggled on', async () => {
    switchEl.selected = true;
    switchEl.dispatchEvent(new Event('change'));

    await waitForRender(explore);
    await new Promise((resolve) => setTimeout(resolve, 0)); // Add delay

    assert.isTrue(explore.state.evenXAxisSpacing);
    assert.isTrue(eventSpy.calledOnce);

    const event = eventSpy.firstCall.args[0];

    assert.equal(event.type, 'even-x-axis-spacing-changed');
    assert.deepEqual(event.detail, { value: true, graph_index: 0 });
  });

  it('should update state and fire event when toggled off', async () => {
    // Turn it on first

    switchEl.selected = true;
    switchEl.dispatchEvent(new Event('change'));

    await waitForRender(explore);

    eventSpy.resetHistory();

    // Turn it off
    switchEl.selected = false;
    switchEl.dispatchEvent(new Event('change'));

    await waitForRender(explore);

    assert.isFalse(explore.state.evenXAxisSpacing);
    assert.isTrue(eventSpy.calledOnce);

    const event = eventSpy.firstCall.args[0];

    assert.equal(event.type, 'even-x-axis-spacing-changed');
    assert.deepEqual(event.detail, { value: false, graph_index: 0 });
  });
});

describe('clearTooltipDataFromURL', () => {
  let explore: ExploreSimpleSk;
  let pushStateStub: sinon.SinonStub;

  beforeEach(async () => {
    explore = setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();
    await fetchMock.flush(true);
  });

  afterEach(() => {
    if (pushStateStub) {
      pushStateStub.restore();
    }
  });

  it('removes graph, commit, and trace params from URL', () => {
    const url = new URL(window.location.href);
    url.searchParams.set('graph', '1');
    url.searchParams.set('commit', '123');
    url.searchParams.set('trace', '456');
    url.searchParams.set('sid', '12345');

    window.history.pushState(null, '', url.toString());

    pushStateStub = sinon.stub(window.history, 'pushState');

    (explore as any).clearTooltipDataFromURL();

    assert.isTrue(pushStateStub.calledOnce);
    // history.pushState(state, title, url)
    const urlPositionIndex = 2;
    const newUrl = new URL(pushStateStub.firstCall.args[urlPositionIndex] as string);
    assert.isNull(newUrl.searchParams.get('graph'));
    assert.isNull(newUrl.searchParams.get('commit'));
    assert.isNull(newUrl.searchParams.get('trace'));
    assert.equal(newUrl.searchParams.get('sid'), '12345');
  });
});

describe('after initial data load', () => {
  let explore: ExploreSimpleSk;

  beforeEach(async () => {
    explore = setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();
    await fetchMock.flush(true);
  });

  describe('Pivot Table Sort', () => {
    it('updates state on sort change', () => {
      const pivotTable = explore.querySelector('pivot-table-sk');
      assert.isNotNull(pivotTable);

      pivotTable!.dispatchEvent(new CustomEvent('change', { detail: 'sort_order' }));
      assert.equal(explore.state.sort, 'sort_order');
    });
  });

  describe('Details Toggle', () => {
    it('toggles navOpen', () => {
      // Force hide_paramset to false so the collapse button is rendered
      explore.state.hide_paramset = false;
      explore.render();

      const collapseButton = explore.querySelector('#collapseButton') as HTMLElement;
      assert.isNotNull(collapseButton);

      assert.isFalse(explore.navOpen);

      collapseButton.click();
      assert.isTrue(explore.navOpen);

      collapseButton.click();
      assert.isFalse(explore.navOpen);
    });
  });
});

describe('Domain Picker Interaction', () => {
  let explore: ExploreSimpleSk;

  beforeEach(async () => {
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
    fetchMock.post('/_/count/', {
      count: 0,
      paramset: {},
    });

    explore = setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();
    await fetchMock.flush(true);
  });

  afterEach(() => {
    fetchMock.reset();
    sinon.restore();
  });

  it('should NOT sync range from domain-picker when useTestPicker is true', async () => {
    (explore as any).useTestPicker = true;
    const initialBegin = 1000;
    const initialEnd = 2000;
    explore.state = {
      ...explore.state,
      begin: initialBegin,
      end: initialEnd,
      queries: ['config=test'],
    };

    // Mock the domain-picker (this.range) to have a LARGER range.
    // The current logic only lengthens the range if the picker's range is wider.
    const pickerBegin = 900;
    const pickerEnd = 2100;
    const mockRange = {
      state: {
        begin: pickerBegin,
        end: pickerEnd,
        num_commits: 50,
        request_type: 0,
      },
    };
    (explore as any).range = mockRange;

    await explore.addFromQueryOrFormula(true, 'query', 'config=test', '');

    assert.equal(explore.state.begin, initialBegin, 'Begin time should not sync with picker');
    assert.equal(explore.state.end, initialEnd, 'End time should not sync with picker');
  });

  it('should sync range from domain-picker when useTestPicker is false', async () => {
    (explore as any).useTestPicker = false;
    const initialBegin = 1000;
    const initialEnd = 2000;
    explore.state = {
      ...explore.state,
      begin: initialBegin,
      end: initialEnd,
      queries: ['config=test'],
    };

    // Mock the domain-picker to have a LARGER range.
    const pickerBegin = 900;
    const pickerEnd = 2100;
    const mockRange = {
      state: {
        begin: pickerBegin,
        end: pickerEnd,
        num_commits: 50,
        request_type: 0,
      },
    };
    (explore as any).range = mockRange;

    await explore.addFromQueryOrFormula(true, 'query', 'config=test', '');

    assert.equal(explore.state.begin, pickerBegin, 'Begin time should sync with picker');
    assert.equal(explore.state.end, pickerEnd, 'End time should sync with picker');
  });

  describe('sendFrameRequest', () => {
    let element: ExploreSimpleSk;
    let dataServiceStub: sinon.SinonStub;

    beforeEach(() => {
      element = new ExploreSimpleSk();
      // Do NOT append to document.body to simulate disconnected state where this.spinner is null.
      dataServiceStub = sinon.stub(DataService.prototype, 'sendFrameRequest');
    });

    afterEach(() => {
      dataServiceStub.restore();
    });

    it('does not crash if element is not connected (spinner is null)', async () => {
      // Setup the stub to call the lifecycle callbacks which interact with this.spinner
      dataServiceStub.callsFake(async (_body: any, options: any) => {
        if (options.onStart) options.onStart();
        if (options.onSettled) options.onSettled();
        return Promise.resolve({
          dataframe: { traceset: {}, header: [], paramset: {}, skip: 0, traceMetadata: [] },
          anomalymap: {},
        });
      });

      const body: FrameRequest = {
        queries: ['test=q'],
        request_type: 1,
        begin: 100,
        end: 200,
        tz: 'US/Pacific',
      };

      // Call the private method
      await (element as any).sendFrameRequest(body);

      assert.isTrue(dataServiceStub.calledOnce);
    });
  });

  describe('Hover Debouncing', () => {
    let explore: ExploreSimpleSk;
    let clock: sinon.SinonFakeTimers;
    let enableTooltipStub: sinon.SinonStub;

    beforeEach(async () => {
      fetchMock.get(/.*\/_\/initpage\/.*/, {
        dataframe: { paramset: {} },
      });
      fetchMock.get('/_/login/status', {
        email: 'someone@example.org',
        roles: ['editor'],
      });

      explore = setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();
      await fetchMock.flush(true);

      clock = sinon.useFakeTimers();
      enableTooltipStub = sinon.stub(explore, 'enableTooltip');

      (explore as any).googleChartPlot = {
        value: {
          getTraceName: () => 'trace1',
          getCommitPosition: () => 100,
          getPositionByIndex: () => ({ x: 10, y: 20 }),
          getYValue: () => 15,
        },
      };
      (explore as any).getCommitDetails = () => ({});
      (explore as any).getPreviousCommit = () => null;
    });

    afterEach(() => {
      clock.restore();
      sinon.restore();
    });

    it('debounces rapid hover events', () => {
      const event = new CustomEvent('plot-data-mouseover', {
        detail: { tableRow: 1, tableCol: 1 },
      });

      // Trigger multiple hovers
      explore['onChartOver'](event as any);
      clock.tick(50);
      explore['onChartOver'](event as any);
      clock.tick(50);
      explore['onChartOver'](event as any);

      assert.isFalse(enableTooltipStub.called, 'Should not be called before 100ms delay');

      clock.tick(100);
      assert.isTrue(enableTooltipStub.calledOnce, 'Should be called once after delay');
    });

    it('cancels pending hover on mouseOut', () => {
      const event = new CustomEvent('plot-data-mouseover', {
        detail: { tableRow: 1, tableCol: 1 },
      });

      explore['onChartOver'](event as any);
      clock.tick(50);

      explore['onChartMouseOut']();
      clock.tick(100);

      assert.isFalse(enableTooltipStub.called, 'Pending hover should be cancelled');
    });

    it('cancels pending hover on mouseDown', () => {
      const event = new CustomEvent('plot-data-mouseover', {
        detail: { tableRow: 1, tableCol: 1 },
      });

      explore['onChartOver'](event as any);
      clock.tick(50);

      explore['onChartMouseDown']();
      clock.tick(100);

      assert.isFalse(enableTooltipStub.called, 'Pending hover should be cancelled');
    });

    it('does not trigger hover if tooltip is already selected', () => {
      const event = new CustomEvent('plot-data-mouseover', {
        detail: { tableRow: 1, tableCol: 1 },
      });

      (explore as any).tooltipSelected = true;
      explore['onChartOver'](event as any);
      clock.tick(150);

      assert.isFalse(
        enableTooltipStub.called,
        'Should not trigger hover logic when tooltip is selected'
      );
    });
  });
});
