import './index';
import { UniformColorSk } from './uniform-color-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('uniform-color-sk', () => {
  const newInstance = setUpElementUnderTest<UniformColorSk>('uniform-color-sk');

  let element: UniformColorSk;
  beforeEach(() => {
    element = newInstance((el: UniformColorSk) => {
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
