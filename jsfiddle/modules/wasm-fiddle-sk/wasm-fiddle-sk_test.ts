import './index';
import { assert } from 'chai';
import {
  colorPickerRegex,
  extractControlNames, sliderRegex,
} from './wasm-fiddle-sk';

describe('wasm-fiddle', () => {
  describe('extractControlNames', () => {
    it('finds all sliders across multiple lines', () => {
      const code = `
#slider1:Foo
#slider2:Bar
      `;
      assert.deepEqual([undefined, 'Foo', 'Bar'], extractControlNames(sliderRegex, code));
    });

    it('finds all sliders on the same line', () => {
      const code = '  #slider1:Foo #slider2:Bar    ';
      assert.deepEqual([undefined, 'Foo', 'Bar'], extractControlNames(sliderRegex, code));
    });

    it('does not crash on empty string', () => {
      const code = '';
      assert.deepEqual([], extractControlNames(sliderRegex, code));
    });

    it('finds all sliders in comments.', () => {
      const code = `  // Comment
  // #slider0:strokeWidth #color0:dashColor
  // #slider1:Bar #color1:Foo
    `;
      assert.deepEqual(['strokeWidth', 'Bar'], extractControlNames(sliderRegex, code), 'sliders');
      assert.deepEqual(['dashColor', 'Foo'], extractControlNames(colorPickerRegex, code), 'color pickers');
    });
  });
});
