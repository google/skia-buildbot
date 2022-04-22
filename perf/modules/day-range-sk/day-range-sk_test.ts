import './index';
import { assert } from 'chai';
import { DayRangeSk } from './day-range-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

const container = document.createElement('div');
document.body.appendChild(container);

afterEach(() => {
  container.innerHTML = '';
});

describe('day-range-sk', () => {
  const newInstance = setUpElementUnderTest<DayRangeSk>(
    'day-range-sk',
  );
  let dayRangeSk: DayRangeSk;
  beforeEach(() => {
    dayRangeSk = newInstance();
  });

  describe('rationalize', () => {
    it('reverses dates if end < begin', () => window.customElements.whenDefined('day-range-sk').then(async () => {
      dayRangeSk.begin = 100;
      dayRangeSk.end = 50;
      dayRangeSk.rationalize();
      assert.equal(dayRangeSk.begin, 50);
      assert.equal(dayRangeSk.end, 100);
    }));

    it('does not reverses dates if begin < end', () => window.customElements.whenDefined('day-range-sk').then(async () => {
      dayRangeSk.begin = 50;
      dayRangeSk.end = 100;
      dayRangeSk.rationalize();
      assert.equal(dayRangeSk.begin, 50);
      assert.equal(dayRangeSk.end, 100);
    }));
  });
});
