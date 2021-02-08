import './index';
import { assert } from 'chai';
import { UniformSliderSk } from './uniform-slider-sk';

import { setUpElementUnderTest } from '../test_util';

describe('uniform-slider-sk', () => {
  const newInstance = setUpElementUnderTest<UniformSliderSk>(
    'uniform-slider-sk',
  );

  let element: UniformSliderSk;
  beforeEach(() => {
    element = newInstance();
  });

  describe('applyUniformValues', () => {
    it('puts value in correct spot in uniforms array', () => {
      // Make uniforms longer than needed to show we don't disturb other values.
      const uniforms: number[] = [0, 0, 0];

      // The control defaults to a value of 0.5.
      element.uniform = {
        name: '',
        columns: 1,
        rows: 1,
        slot: 1,
      };
      element.applyUniformValues(uniforms);
      assert.deepEqual(uniforms, [0, 0.5, 0]);
    });

    it('retores value in correct spot in uniforms array', () => {
      // The control defaults to a value of 0.5.
      element.uniform = {
        name: '',
        columns: 1,
        rows: 1,
        slot: 1,
      };
      element.restoreUniformValues([0, 0.4, 0]);

      const uniforms: number[] = [0, 0, 0];
      element.applyUniformValues(uniforms);
      assert.deepEqual(uniforms, [0, 0.4, 0]);
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
