import './index';
import { ScrapexchangeSk } from './scrapexchange-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('scrapexchange-sk', () => {
  const newInstance = setUpElementUnderTest<ScrapexchangeSk>('scrapexchange-sk');

  let element: ScrapexchangeSk;
  beforeEach(() => {
    element = newInstance((el: ScrapexchangeSk) => {
      // Place here any code that must run after the element is instantiated but
      // before it is attached to the DOM (e.g. property setter calls,
      // document-level event listeners, etc.).
    });
  });

  describe('some action', () => {
    it('some result', () => {});
      expect(element).to.not.be.null;
  });
});
