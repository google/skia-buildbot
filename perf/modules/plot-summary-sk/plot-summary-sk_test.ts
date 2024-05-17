import './index';
import { expect } from 'chai';
import { PlotSummarySk } from './plot-summary-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('plot-summary-sk', () => {
  const newInstance = setUpElementUnderTest<PlotSummarySk>('plot-summary-sk');

  let element: PlotSummarySk;
  beforeEach(() => {
    element = newInstance((el: PlotSummarySk) => {
      // Place here any code that must run after the element is instantiated but
      // before it is attached to the DOM (e.g. property setter calls,
      // document-level event listeners, etc.).
    });
  });

  describe('some action', () => {
    it('some result', () => {
      expect(element).to.not.be.null;
    });
  });
});
