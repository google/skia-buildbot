import './index';
import { DebuggerPageSk } from './debugger-page-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('debugger-page-sk', () => {
  const newInstance = setUpElementUnderTest<DebuggerPageSk>('debugger-page-sk');

  let element: DebuggerPageSk;
  beforeEach(() => {
    element = newInstance((el: DebuggerPageSk) => {
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
