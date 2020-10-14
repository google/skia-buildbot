import './index';
import { AutorollerStatusSk } from './autoroller-status-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('autoroller-status-sk', () => {
  const newInstance = setUpElementUnderTest<AutorollerStatusSk>('autoroller-status-sk');

  let element: AutorollerStatusSk;
  beforeEach(() => {
    element = newInstance((el: AutorollerStatusSk) => {
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
