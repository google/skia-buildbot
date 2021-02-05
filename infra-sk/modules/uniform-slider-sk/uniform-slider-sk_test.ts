import './index';
import { UniformSliderSk } from './uniform-slider-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { assert } from 'chai';

describe('uniform-slider-sk', () => {
  const newInstance = setUpElementUnderTest<UniformSliderSk>(
    'uniform-slider-sk'
  );

  let element: UniformSliderSk;
  beforeEach(() => {
    element = newInstance((el: UniformSliderSk) => {
      // Place here any code that must run after the element is instantiated but
      // before it is attached to the DOM (e.g. property setter calls,
      // document-level event listeners, etc.).
    });
  });

  describe('applyUniformValues', () => {
    it('puts value in correct spot in uniforms array', () => {
      // Make uniforms longer than needed to show we don't disturb other values.
      const uniforms = new Float32Array(3);

      // The control defaults to a value of 0.5.
      element.uniform = {
        name: '',
        columns: 1,
        rows: 1,
        slot: 1,
      };
      element.applyUniformValues(uniforms);
      assert.deepEqual(uniforms, new Float32Array([0, 0.5, 0]));
    });
    it('throws on invalid uniforms', () => {
      assert.throws(() => {
        // Rows and columns must both equal 1.
        element.uniform = {
          name: '',
          columns: 2,
          rows: 2,
          slot: 1,
        };
      });
    });
  });
});
