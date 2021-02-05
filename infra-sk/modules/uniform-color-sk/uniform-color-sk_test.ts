import './index';
import { UniformColorSk } from './uniform-color-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { assert } from 'chai';
import { $$ } from 'common-sk/modules/dom';

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
    it('puts values in correct spot in uniforms array', () => {
      // Make uniforms longer than needed to show we don't disturb other values.
      const uniforms = new Float32Array(5);

      element.uniform = {
        name: 'color',
        columns: 3,
        rows: 1,
        slot: 1,
      };

      // Set the color to white, which gives us all ones as output uniforms.
      $$<HTMLInputElement>('input', element)!.value = '#8090a0';
      element.applyUniformValues(uniforms);
      assert.deepEqual(
        uniforms,
        new Float32Array([0, 128 / 255, 144 / 255, 160 / 255, 0])
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
