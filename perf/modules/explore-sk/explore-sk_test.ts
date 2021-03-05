/* eslint-disable dot-notation */
import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import { FrameRequest, progress } from '../json';
import { calculateRangeChange, ExploreSk } from './explore-sk';

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
  window.sk = {
    perf: {
      radius: 2,
      key_order: null,
      num_shift: 50,
      interesting: 2,
      step_up_only: false,
      commit_range_url: '',
      demo: true,
      display_group_by: false,
    },
  };

  // Create a common element-sk to be used by all the tests.
  const explore = document.createElement('explore-sk') as ExploreSk;
  document.body.appendChild(explore);

  const finishedBody: progress.SerializedProgress = {
    status: 'Finished',
    messages: [{ key: 'Step', value: '2/2' }],
    results: { somedata: 1 },
    url: '',
  };

  it('applies the func to existing formulas', async () => {
    const startURL = '/_/frame/start';
    fetchMock.post(startURL, finishedBody);
    explore['state'].formulas = ['shortcut("Xfoo")'];
    await explore['applyFuncToTraces']('iqrr');
    assert.isTrue(fetchMock.done());
    const body = JSON.parse(fetchMock.lastOptions(startURL)?.body as unknown as string);
    assert.deepEqual(body.formulas, ['iqrr(shortcut("Xfoo"))']);
    fetchMock.restore();
  });
});
