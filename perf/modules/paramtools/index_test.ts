import { assert } from 'chai';
import { ParamSet } from '../json';
import {
  addParamSet,
  addParamsToParamSet,
  fromKey,
  makeKey,
  paramsToParamSet,
  validKey,
} from './index';

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

describe('paramsToParamSet', () => {
  it('handles empy Params', () => {
    assert.deepEqual({}, paramsToParamSet({}));
  });

  it('handles a single Param', () => {
    assert.deepEqual({ a: ['b'] }, paramsToParamSet({ a: 'b' }));
  });

  it('handles multiple Params', () => {
    assert.deepEqual(
      { a: ['1'], b: ['2'] },
      paramsToParamSet({ a: '1', b: '2' })
    );
  });
});

describe('validKey', () => {
  it('returns true for valid trace ids', () => {
    assert.isTrue(validKey(',a=b,'));
  });

  it('returns false for calculations', () => {
    assert.isFalse(validKey('avg(,a=b,)'));
  });
});

describe('addParamSet', () => {
  it('adds one param to set of two params', () => {
    const a: ParamSet = { foo: ['a', 'b'] };
    const b: ParamSet = { foo: ['c'] };
    addParamSet(a, b);
    assert.deepEqual(a, { foo: ['a', 'b', 'c'] });
  });

  it('adds one param to empty params', () => {
    const a: ParamSet = {};
    const b: ParamSet = { foo: ['c'] };
    addParamSet(a, b);
    assert.deepEqual(a, { foo: ['c'] });
  });

  it('adds empty params to set of one params', () => {
    const a: ParamSet = { foo: ['c'] };
    const b: ParamSet = {};
    addParamSet(a, b);
    assert.deepEqual(a, { foo: ['c'] });
  });

  it('adds params for disjoint key sets', () => {
    const a: ParamSet = { foo: ['c'] };
    const b: ParamSet = { bar: ['b'] };
    addParamSet(a, b);
    assert.deepEqual(a, { foo: ['c'], bar: ['b'] });
  });
});
