/* eslint-disable dot-notation */
import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import {
  ColumnHeader,
  CommitNumber,
  QueryConfig,
  TimestampSeconds,
  Trace,
  TraceSet,
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
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { generateFullDataFrame } from '../dataframe/test_utils';
import { UserIssueMap } from '../dataframe/dataframe_context';

fetchMock.config.overwriteRoutes = true;

const now = 1726081856; // an arbitrary UNIX time;
const timeSpan = 89; // an arbitrary prime number for time span between commits .

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

// this function is needed to support other unit tests
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
    keys_for_useful_links: [],
    skip_commit_detail_display: false,
    image_tag: 'fake-tag',
    remove_default_stat_value: false,
    show_json_file_display: false,
    always_show_commit_info: false,
  };

  // Create a common element-sk to be used by all the tests.
  const explore = document.createElement('explore-simple-sk') as ExploreSimpleSk;
  document.body.appendChild(explore);
});

describe('addGraphCoordinatesToUserIssues', () => {
  it('adds plot coordinates to user issues', () => {
    const explore = setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();
    const keys = [
      ',benchmark=JetStream2,bot=MacM1,ref_mode=head,subtest=Average,test=Total,v8_mode=pgo,',
      ',benchmark=JetStream2,bot=MacM1,ref_mode=ref,subtest=Average,test=Total,v8_mode=default,',
      ',benchmark=JetStream2,bot=MacM1,ref_mode=ref,subtest=Normal,test=Total,v8_mode=default,',
      ',benchmark=JetStream2,bot=MacM1,ref_mode=ref,subtest=Normal,test=Total,',
    ];
    const df = generateFullDataFrame(
      { begin: 90, end: 120 },
      now,
      keys.length,
      [timeSpan],
      [Array.from({ length: 4 }, (_, k) => k)],
      keys
    );

    const trace =
      ',benchmark=JetStream2,bot=MacM1,ref_mode=ref,subtest=Normal,test=Total,v8_mode=default,';
    const offset = df.header![1]?.offset || -1;
    const value = df.traceset![trace][1];

    const userIssues: UserIssueMap = {};
    userIssues[trace] = {};
    userIssues[trace][offset] = { bugId: 12345, x: -1, y: -1 };

    const updatedIssues = explore['addGraphCoordinatesToUserIssues'](df, userIssues);

    const expected: UserIssueMap = {};
    expected[trace] = {};
    expected[trace][offset] = { bugId: 12345, x: 1, y: value };

    assert.deepEqual(updatedIssues, expected);
  });

  it('does not add plot coordinates to user issues for points not in df', () => {
    const explore = setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();
    const keys = [
      ',benchmark=JetStream2,bot=MacM1,ref_mode=head,subtest=Average,test=Total,v8_mode=pgo,',
      ',benchmark=JetStream2,bot=MacM1,ref_mode=ref,subtest=Average,test=Total,v8_mode=default,',
      ',benchmark=JetStream2,bot=MacM1,ref_mode=ref,subtest=Normal,test=Total,v8_mode=default,',
      ',benchmark=JetStream2,bot=MacM1,ref_mode=ref,subtest=Normal,test=Total,',
    ];
    const df = generateFullDataFrame(
      { begin: 90, end: 120 },
      now,
      keys.length,
      [timeSpan],
      [Array.from({ length: 4 }, (_, k) => k)],
      keys
    );

    const trace =
      ',benchmark=JetStream2,bot=MacM1,ref_mode=ref,subtest=Normal,test=Total,v8_mode=default,';
    const offset = 56789;

    const userIssues: UserIssueMap = {};
    userIssues[trace] = {};
    userIssues[trace][offset] = { bugId: 12345, x: -1, y: -1 };

    const updatedIssues = explore['addGraphCoordinatesToUserIssues'](df, userIssues);

    const expected: UserIssueMap = {};
    expected[trace] = {};
    expected[trace][offset] = { bugId: 12345, x: -1, y: -1 };

    assert.deepEqual(updatedIssues, expected);
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
