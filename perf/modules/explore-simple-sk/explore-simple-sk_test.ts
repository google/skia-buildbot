/* eslint-disable dot-notation */
import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import { ColumnHeader, QueryConfig, progress } from '../json';
import { deepCopy } from '../../../infra-sk/modules/object';
import {
  calculateRangeChange,
  defaultPointSelected,
  ExploreSimpleSk,
  isValidSelection,
  PointSelected,
  selectionToEvent,
} from './explore-simple-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

fetchMock.config.overwriteRoutes = true;

describe('calculateRangeChange', () => {
  const offsets: [number, number] = [100, 120];

  it('finds a left range increase', () => {
    const zoom: [number, number] = [-1, 10];
    const clampedZoom: [number, number] = [0, 10];

    const ret = calculateRangeChange(zoom, clampedZoom, offsets);
    assert.isTrue(ret.rangeChange);

    // We shift the beginning of the range by RANGE_CHANGE_ON_ZOOM_PERCENT of
    // the total range.
    assert.deepEqual(ret.newOffsets, [90, 120]);
  });

  it('finds a right range increase', () => {
    const zoom: [number, number] = [0, 12];
    const clampedZoom: [number, number] = [0, 10];

    const ret = calculateRangeChange(zoom, clampedZoom, offsets);
    assert.isTrue(ret.rangeChange);

    // We shift the end of the range by RANGE_CHANGE_ON_ZOOM_PERCENT of the
    // total range.
    assert.deepEqual(ret.newOffsets, [100, 130]);
  });

  it('find an increase in the range in both directions', () => {
    const zoom: [number, number] = [-1, 11];
    const clampedZoom: [number, number] = [0, 10];

    const ret = calculateRangeChange(zoom, clampedZoom, offsets);
    assert.isTrue(ret.rangeChange);

    // We shift both the begin and end of the range by
    // RANGE_CHANGE_ON_ZOOM_PERCENT of the total range.
    assert.deepEqual(ret.newOffsets, [90, 130]);
  });

  it('find an increase in the range in both directions and clamps the offset', () => {
    const zoom: [number, number] = [-1, 11];
    const clampedZoom: [number, number] = [0, 10];
    const widerOffsets: [number, number] = [0, 100];

    const ret = calculateRangeChange(zoom, clampedZoom, widerOffsets);
    assert.isTrue(ret.rangeChange);

    // We shift both the begin and end of the range by
    // RANGE_CHANGE_ON_ZOOM_PERCENT of the total range.
    assert.deepEqual(ret.newOffsets, [0, 150]);
  });

  it('does not find a range change', () => {
    const zoom: [number, number] = [0, 10];
    const clampedZoom: [number, number] = [0, 10];

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
        offset: 99,
        timestamp: 0,
      },
      {
        offset: 100,
        timestamp: 0,
      },
      {
        offset: 101,
        timestamp: 0,
      },
    ];

    const p: PointSelected = {
      commit: 100,
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
        offset: 99,
        timestamp: 0,
      },
      {
        offset: 100,
        timestamp: 0,
      },
      {
        offset: 101,
        timestamp: 0,
      },
    ];

    const p: PointSelected = {
      commit: 102,
      name: 'foo',
    };
    // selectionToEvent will look up the commit (aka offset) in header and
    // should return an event where the 'x' value is -1 since the matching
    // ColumnHeader in 'header' doesn't exist.
    const e = selectionToEvent(p, header);
    assert.equal(e.detail.x, -1);
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

    var explore = await setUpElementUnderTest<ExploreSimpleSk>(
      'explore-simple-sk'
    )();
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
    const defaultBody = JSON.stringify(defaultConfig);
    fetchMock.get('path:/_/defaults/', {
      status: 200,
      body: defaultBody,
    });

    var explore = setUpElementUnderTest<ExploreSimpleSk>('explore-simple-sk')();
    await fetchMock.flush(true);
    var actualConfig = explore['defaults'];
    assert.deepEqual(actualConfig, defaultConfig);
    const originalState = deepCopy(explore.state);
    await explore['applyQueryDefaultsIfMissing']();
    assert.isTrue(fetchMock.done());

    const newState = explore.state;
    assert.notDeepEqual(newState, originalState);
    assert.isTrue(newState.summary);
  });
});
