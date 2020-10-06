import './index';
import { StatusSk } from './status-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('status-sk', () => {
  const newInstance = setUpElementUnderTest<StatusSk>('status-sk');

  let element: StatusSk;
  beforeEach(() => {
    element = newInstance((el: StatusSk) => {
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
