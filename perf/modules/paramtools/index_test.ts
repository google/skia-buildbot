import { assert } from 'chai';
import { ParamSet } from '../json';
import { addParamsToParamSet, fromKey, makeKey } from './index';

describe('paramtooms', () => {
  describe('makeKey', () => {
    it('constructs a key correctly', () => {
      assert.equal(',a=1,b=2,c=3,', makeKey({ b: '2', a: '1', c: '3' }));
    });

    it('throws on empty Params', () => {
      assert.throw(() => makeKey({}));
    });
  });

  describe('fromKey', () => {
    it('parses a key correctly', () => {
      assert.deepEqual({ b: '2', a: '1', c: '3' }, fromKey(',a=1,b=2,c=3,'));
    });

    it('handles empty string as key', () => {
      assert.deepEqual({}, fromKey(''));
    });
  });

  describe('addParamsToParamSet', () => {
    it('works on empty values', () => {
      const ps: ParamSet = {};
      addParamsToParamSet(ps, {});
      assert.deepEqual({}, ps);
    });

    it('handles duplicate keys and values', () => {
      const ps: ParamSet = {};
      addParamsToParamSet(ps, { a: '1' });
      addParamsToParamSet(ps, { a: '1' });
      assert.deepEqual({ a: ['1'] }, ps);
    });

    it('handles distinct keys and values', () => {
      const ps: ParamSet = {};
      addParamsToParamSet(ps, { a: '1' });
      addParamsToParamSet(ps, { b: '2' });
      assert.deepEqual({ a: ['1'], b: ['2'] }, ps);
    });

    it('handles distinct keys and multiple values', () => {
      const ps: ParamSet = {};
      addParamsToParamSet(ps, { a: '1' });
      addParamsToParamSet(ps, { b: '2' });
      addParamsToParamSet(ps, { b: '3' });
      assert.deepEqual({ a: ['1'], b: ['2', '3'] }, ps);
    });
  });
});
