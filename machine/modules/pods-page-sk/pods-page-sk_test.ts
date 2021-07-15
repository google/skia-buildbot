import './index';
import { PodsPageSk } from './pods-page-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('pods-page-sk', () => {
  const newInstance = setUpElementUnderTest<PodsPageSk>('pods-page-sk');

  let element: PodsPageSk;
  beforeEach(() => {
    element = newInstance((el: PodsPageSk) => {
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
