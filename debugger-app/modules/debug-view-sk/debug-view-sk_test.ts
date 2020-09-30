import './index';
import { DebugViewSk } from './debug-view-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('debug-view-sk', () => {
  const newInstance = setUpElementUnderTest<DebugViewSk>('debug-view-sk');

  let element: DebugViewSk;
  beforeEach(() => {
    element = newInstance((el: DebugViewSk) => {
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
