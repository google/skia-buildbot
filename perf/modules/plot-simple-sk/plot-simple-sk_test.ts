import './index';
import { assert } from 'chai';
import { PlotSimpleSk } from './plot-simple-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('example-control-sk', () => {
  const newInstance = setUpElementUnderTest<PlotSimpleSk>('plot-simple-sk');

  let element: PlotSimpleSk;
  beforeEach(() => {
    element = newInstance((el: PlotSimpleSk) => {
      // Place here any code that must run after the element is instantiated but
      // before it is attached to the DOM (e.g. property setter calls,
      // document-level event listeners, etc.).
    });
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
});
