import './index';
import { ScrapExchangeSk } from './scrap-exchange-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('scrap-exchange-sk', () => {
  const newInstance = setUpElementUnderTest<ScrapExchangeSk>('scrap-exchange-sk');

  let element: ScrapExchangeSk;
  beforeEach(() => {
    element = newInstance((el: ScrapExchangeSk) => {
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
