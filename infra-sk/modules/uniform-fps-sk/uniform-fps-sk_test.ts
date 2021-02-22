import './index';
import { assert } from 'chai';
import { UniformFpsSk } from './uniform-fps-sk';

import { setUpElementUnderTest } from '../test_util';

describe('uniform-fps-sk', () => {
  const newInstance = setUpElementUnderTest<UniformFpsSk>('uniform-fps-sk');

  let element: UniformFpsSk;
  beforeEach(() => {
    element = newInstance();
  });

  describe('uniform-fps-sk', () => {
    it('needs raf updates', () => {
      assert.isTrue(element.needsRAF());
    });

    it('updates on onRAF()', () => {
      assert.equal(element.textContent, 'fps');
      element.onRAF();
      assert.equal(element.textContent, '0 fps');
    });
  });
});
