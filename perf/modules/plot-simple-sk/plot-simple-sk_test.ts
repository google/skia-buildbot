import './index';
import { assert } from 'chai';
import * as d3Scale from 'd3-scale';
import {
  PlotSimpleSk, Range, rectFromRange, rectFromRangeInvert,
} from './plot-simple-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('plot-simple-sk', () => {
  const newInstance = setUpElementUnderTest<PlotSimpleSk>('plot-simple-sk');

  let element: PlotSimpleSk;
  beforeEach(() => {
    element = newInstance();
  });

  describe('add some lines to plot', () => {
    it('lists those lines, but not special_zero', () => {
      element.addLines({ line1: [1, 2, 3] }, [new Date(2020, 4, 1), new Date(2020, 4, 2), new Date(2020, 4, 3)]);
      element.addLines({ special_zero: [0, 0, 0] }, []);
      assert.deepEqual(['line1'], element.getLineNames());
    });
  });

  describe('do not add lines to plot', () => {
    it('getLineNames returns empty list', () => {
      assert.deepEqual([], element.getLineNames());
    });
  });

  describe('rectFromRange', () => {
    it('roundtrips through rectFromRangeInvert correctly', () => {
      const range: Range = {
        x: d3Scale.scaleLinear().domain([0, 100]).range([0, 1024]),
        y: d3Scale.scaleLinear().domain([100, 120]).range([0, 512]),
      };
      // Start with a rectangle that looks like one drawn on canvas from top
      // left to bottom right, which means that y actually goes from a high
      // value to a low value.
      const rect = {
        x: 0,
        y: 100,
        width: 100,
        height: -100,
      };
      const results = rectFromRange(range, rect);
      const back = rectFromRangeInvert(range, results);
      assert.equal(back.x, rect.x, 'x');
      assert.equal(back.y, rect.y, 'y');
      assert.equal(back.width, rect.width, 'width');
      assert.equal(back.height, rect.height, 'height');
    });
  });
});
