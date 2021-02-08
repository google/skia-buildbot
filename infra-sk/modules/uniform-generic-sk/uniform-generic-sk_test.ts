import './index';
import { assert } from 'chai';
import { $$ } from 'common-sk/modules/dom';
import { UniformGenericSk } from './uniform-generic-sk';

import { setUpElementUnderTest } from '../test_util';

describe('uniform-generic-sk', () => {
  const newInstance = setUpElementUnderTest<UniformGenericSk>('uniform-generic-sk');

  let element: UniformGenericSk;
  beforeEach(() => {
    element = newInstance();
  });

  describe('uniform-generic-sk', () => {
    it('handles non-square uniforms', () => {
      // Make uniforms longer than needed to show we don't disturb other values.
      const uniforms: number[] = [0, 0, 0, 0, 0, 0, 0, 0];

      // The control defaults to values of 0.5.
      element.uniform = {
        name: 'nonsquare',
        columns: 3,
        rows: 2,
        slot: 1,
      };
      element.applyUniformValues(uniforms);
      assert.deepEqual(uniforms, [0, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0]);
    });

    it('handles square uniforms', () => {
      // Make uniforms longer than needed to show we don't disturb other values.
      const uniforms: number[] = [0, 0, 0, 0, 0, 0];

      // The control defaults to the identity matrix for square uniforms.
      element.uniform = {
        name: 'square',
        columns: 2,
        rows: 2,
        slot: 1,
      };
      element.applyUniformValues(uniforms);
      assert.deepEqual(uniforms, [0, 1, 0, 0, 1, 0]);
    });

    it('applies uniform values in column major order', () => {
      // Make uniforms longer than needed to show we don't disturb other values.
      const uniforms: number[] = [0, 0, 0, 0, 0, 0];

      // The control defaults to the identity matrix for square uniforms.
      element.uniform = {
        name: 'square',
        columns: 2,
        rows: 2,
        slot: 1,
      };
      $$<HTMLInputElement>('#square_1_0', element)!.value = '0.5'; // row=1 col=0
      element.applyUniformValues(uniforms);
      assert.deepEqual(uniforms, [0, 1, 0.5, 0, 1, 0]);
    });

    it('restores uniform values in column major order', () => {
      // The control defaults to the identity matrix for square uniforms.
      element.uniform = {
        name: 'square',
        columns: 2,
        rows: 2,
        slot: 1,
      };
      element.restoreUniformValues([0, 1, 0.5, 0.3, 1, 0]);
      const uniforms: number[] = [0, 0, 0, 0, 0, 0];
      element.applyUniformValues(uniforms);
      assert.deepEqual(uniforms, [0, 1, 0.5, 0.3, 1, 0]);
    });
  });
});
