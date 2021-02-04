import './index';
import { UniformSliderSk } from './uniform-slider-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('uniform-slider-sk', () => {
  const newInstance = setUpElementUnderTest<UniformSliderSk>('uniform-slider-sk');

  let element: UniformSliderSk;
  beforeEach(() => {
    element = newInstance((el: UniformSliderSk) => {
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
