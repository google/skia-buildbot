import './index';
import { AutoRefreshSk } from './auto-refresh-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('auto-refresh-sk', () => {
  const newInstance = setUpElementUnderTest<AutoRefreshSk>('auto-refresh-sk');

  let element: AutoRefreshSk;
  beforeEach(() => {
    element = newInstance((el: AutoRefreshSk) => {
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
