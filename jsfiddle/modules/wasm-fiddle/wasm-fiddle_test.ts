import './index';
import { assert } from 'chai';
import {
  pickNames,
} from './wasm-fiddle';

describe('wasm-fiddle', () => {
  describe('pickNames', () => {
    it('finds all sliders', () => {
      const code = `  
#slider1:Foo
#slider2:Bar
      `;
      assert.deepEqual([undefined, 'Foo', 'Bar'], pickNames(/#slider(\d):(\S+)/g, code));
    });
  });
});
