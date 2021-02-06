import './index';
import { assert } from 'chai';
import { $$ } from 'common-sk/modules/dom';
import { UniformColorSk } from './uniform-color-sk';
import { setUpElementUnderTest } from '../test_util';

describe('uniform-color-sk', () => {
  const newInstance = setUpElementUnderTest<UniformColorSk>('uniform-color-sk');

  let element: UniformColorSk;
  beforeEach(() => {
    element = newInstance();
  });

  describe('some action', () => {
    it('puts values in correct spot in uniforms array', () => {
      // Make uniforms longer than needed to show we don't disturb other values.
      const uniforms = new Float32Array(5);

      element.uniform = {
        name: 'color',
        columns: 3,
        rows: 1,
        slot: 1,
      };

      $$<HTMLInputElement>('input', element)!.value = '#8090a0';
      element.applyUniformValues(uniforms);
      assert.deepEqual(
        uniforms,
        new Float32Array([0, 128 / 255, 144 / 255, 160 / 255, 0]),
      );
    });

    it('puts values in correct spot in uniforms array with an alpha', () => {
      // Make uniforms longer than needed to show we don't disturb other values.
      const uniforms = new Float32Array(6);

      element.uniform = {
        name: 'color',
        columns: 4, // float4 implies an alpha channel.
        rows: 1,
        slot: 1,
      };

      $$<HTMLInputElement>('input', element)!.value = '#8090a0';
      element.applyUniformValues(uniforms);
      assert.deepEqual(
        uniforms,
        new Float32Array([0, 128 / 255, 144 / 255, 160 / 255, 0.5, 0]),
      );
    });

    it('throws on invalid uniforms', () => {
      // Uniform is too small to be a color.
      assert.throws(() => {
        element.uniform = {
          name: 'color',
          columns: 1,
          rows: 1,
          slot: 1,
        };
      });
    });
  });
});
