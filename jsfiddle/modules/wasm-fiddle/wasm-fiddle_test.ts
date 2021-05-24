import './index';
import { assert } from 'chai';
import {
  pickNames,
} from './wasm-fiddle';

describe('wasm-fiddle', () => {
  describe('pickNames', () => {
    it('finds all sliders across multiple lines', () => {
      const code = `  
#slider1:Foo
#slider2:Bar
      `;
      assert.deepEqual([undefined, 'Foo', 'Bar'], pickNames(/#slider(\d):(\S+)/g, code));
    });

    it('finds all sliders on the same line', () => {
      const code = '  #slider1:Foo #slider2:Bar    ';
      assert.deepEqual([undefined, 'Foo', 'Bar'], pickNames(/#slider(\d):(\S+)/g, code));
    });

    it('does not crash on empty string', () => {
      const code = '';
      assert.deepEqual([], pickNames(/#slider(\d):(\S+)/g, code));
    });
  });
});
