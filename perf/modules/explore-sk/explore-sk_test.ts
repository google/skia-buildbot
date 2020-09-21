import { assert } from 'chai';
import { calculateRangeChange } from './explore-sk';

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
