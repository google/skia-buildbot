import './index';
import { assert } from 'chai';
import { $$ } from 'common-sk/modules/dom';
import { hexToSlot, slotToHex, UniformColorSk } from './uniform-color-sk';
import { setUpElementUnderTest } from '../test_util';

describe('uniform-color-sk', () => {
  const newInstance = setUpElementUnderTest<UniformColorSk>('uniform-color-sk');

  let element: UniformColorSk;
  beforeEach(() => {
    element = newInstance();
  });

  describe('slotToHex', () => {
    it('prepends a zero to make a two digit hex value', () => {
      assert.equal(slotToHex([4 / 255], 0), '04');
      assert.equal(slotToHex([10 / 255], 0), '0a');
    });
  });

  describe('hexToSlot', () => {
    it('converts the two digit hex into a float and stores it in the right slot.', () => {
      const uniforms = [0, 0, 0];
      hexToSlot('05', uniforms, 1);
      assert.deepEqual(uniforms, [0, 0.0196, 0]);
    });
  });

  describe('hexToSlot and slotToHex', () => {
    it('roundtrip correctly from number to hex and back to number', () => {
      const uniforms = [0, 0, 0];
      for (let i = 0; i < 255; i++) {
        // Convert a known hex and place it in slot 1.
        hexToSlot(i.toString(16), uniforms, 1);

        // Now roundtrip that value out and back into slot 2.
        const hex = slotToHex(uniforms, 1);
        hexToSlot(hex, uniforms, 2);

        assert.equal(uniforms[1], uniforms[2]);
      }
    });
  });

  describe('uniform-color-sk', () => {
    it('puts values in correct spot in uniforms array', () => {
      // Make uniforms longer than needed to show we don't disturb other values.
      const uniforms = [0, 0, 0, 0, 0];

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
        [0, 0.5019, 0.5647, 0.6274, 0],
      );
    });

    it('puts values in correct spot in uniforms array with an alpha', () => {
      // Make uniforms longer than needed to show we don't disturb other values.
      const uniforms: number[] = [0, 0, 0, 0, 0, 0];

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
        [0, 0.5019, 0.5647, 0.6274, 0.5, 0],
      );
    });

    it('restores values in correct spot in uniforms array with an alpha', () => {
      // Make uniforms longer than needed to show we don't disturb other values.
      let uniforms: number[] = [0, 128 / 255, 144 / 255, 160 / 255, 0.5, 0];

      element.uniform = {
        name: 'color',
        columns: 4, // float4 implies an alpha channel.
        rows: 1,
        slot: 1,
      };

      $$<HTMLInputElement>('input', element)!.value = '#8090a0';
      element.restoreUniformValues(uniforms);

      // Clear uniforms.
      uniforms = [0, 0, 0, 0, 0, 0];
      element.applyUniformValues(uniforms);
      assert.deepEqual(
        uniforms,
        [0, 0.5019, 0.5647, 0.6274, 0.5, 0],
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
