import './index';
import { UniformGenericSk } from './uniform-generic-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('uniform-generic-sk', () => {
  const newInstance = setUpElementUnderTest<UniformGenericSk>('uniform-generic-sk');

  let element: UniformGenericSk;
  beforeEach(() => {
    element = newInstance((el: UniformGenericSk) => {
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
