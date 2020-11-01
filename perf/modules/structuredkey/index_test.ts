import { assert } from 'chai';
import { makeKey } from './index';

describe('structuredkey', () => {
  describe('makeKey', () => {
    it('constructs a key correctly', () => {
      assert.equal(',a=1,b=2,c=3,', makeKey({ b: '2', a: '1', c: '3' }));
    });

    it('throws on empty Params', () => {
      assert.throw(() => makeKey({}));
    });
  });
});
