import './index';
import { DayRangeSk, DayRangeSkChangeDetail } from './day-range-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { assert } from 'chai';
import * as sinon from 'sinon';

describe('day-range-sk', () => {
  const newInstance = setUpElementUnderTest<DayRangeSk>('day-range-sk');

  let element: DayRangeSk;
  let clock: sinon.SinonFakeTimers;

  beforeEach(() => {
    // Set a fixed time: 2026-01-12T12:00:00Z
    clock = sinon.useFakeTimers(new Date('2026-01-12T12:00:00Z').getTime());
    element = newInstance();
  });

  afterEach(() => {
    clock.restore();
  });

  it('renders', () => {
    assert.isNotNull(element);
  });

  it('defaults to last 24 hours', () => {
    const begin = element.begin;
    const end = element.end;
    const nowInSeconds = Math.floor(Date.now() / 1000);
    // Should be exactly 24 hours difference
    assert.equal(end, nowInSeconds);
    assert.equal(begin, nowInSeconds - 24 * 60 * 60);
  });

  it('emits event when begin calendar input changes', async () => {
    const beginInput = element.querySelector('.begin calendar-input-sk')!;
    const eventPromise = new Promise<DayRangeSkChangeDetail>((resolve) => {
      element.addEventListener(
        'day-range-change',
        (e) => {
          resolve((e as CustomEvent<DayRangeSkChangeDetail>).detail);
        },
        { once: true }
      );
    });

    const date = new Date(1600000000000);
    beginInput.dispatchEvent(
      new CustomEvent('input', {
        detail: date,
        bubbles: true,
      })
    );

    const detail = await eventPromise;
    assert.equal(detail.begin, 1600000000);
    // End should remain unchanged (now)
    assert.equal(detail.end, Math.floor(Date.now() / 1000));
  });

  it('emits event when end calendar input changes', async () => {
    const endInput = element.querySelector('.end calendar-input-sk')!;
    const eventPromise = new Promise<DayRangeSkChangeDetail>((resolve) => {
      element.addEventListener(
        'day-range-change',
        (e) => {
          resolve((e as CustomEvent<DayRangeSkChangeDetail>).detail);
        },
        { once: true }
      );
    });

    const date = new Date(1600000000000);
    endInput.dispatchEvent(
      new CustomEvent('input', {
        detail: date,
        bubbles: true,
      })
    );

    const detail = await eventPromise;
    assert.equal(detail.end, 1600000000);
    // Begin should remain unchanged (now - 24h)
    assert.equal(detail.begin, Math.floor(Date.now() / 1000) - 24 * 60 * 60);
  });
});
