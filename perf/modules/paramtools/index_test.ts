import { assert } from 'chai';
import { ParamSet, Params } from '../json';
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
      assert.deepEqual(
        Params({ b: '2', a: '1', c: '3' }),
        fromKey(',a=1,b=2,c=3,')
      );
    });

    it('handles empty string as key', () => {
      assert.deepEqual(Params({}), fromKey(''));
    });
  });

  describe('addParamsToParamSet', () => {
    it('works on empty values', () => {
      const ps = ParamSet({});
      addParamsToParamSet(ps, Params({}));
      assert.deepEqual(ParamSet({}), ps);
    });

    it('handles duplicate keys and values', () => {
      const ps = ParamSet({});
      addParamsToParamSet(ps, Params({ a: '1' }));
      addParamsToParamSet(ps, Params({ a: '1' }));
      assert.deepEqual(ParamSet({ a: ['1'] }), ps);
    });

    it('handles distinct keys and values', () => {
      const ps = ParamSet({});
      addParamsToParamSet(ps, Params({ a: '1' }));
      addParamsToParamSet(ps, Params({ b: '2' }));
      assert.deepEqual(ParamSet({ a: ['1'], b: ['2'] }), ps);
    });

    it('handles distinct keys and multiple values', () => {
      const ps = ParamSet({});
      addParamsToParamSet(ps, Params({ a: '1' }));
      addParamsToParamSet(ps, Params({ b: '2' }));
      addParamsToParamSet(ps, Params({ b: '3' }));
      assert.deepEqual(ParamSet({ a: ['1'], b: ['2', '3'] }), ps);
    });
  });
});

describe('paramsToParamSet', () => {
  it('handles empy Params', () => {
    assert.deepEqual(ParamSet({}), paramsToParamSet(Params({})));
  });

  it('handles a single Param', () => {
    assert.deepEqual(
      ParamSet({ a: ['b'] }),
      paramsToParamSet(Params({ a: 'b' }))
    );
  });

  it('handles multiple Params', () => {
    assert.deepEqual(
      ParamSet({ a: ['1'], b: ['2'] }),
      paramsToParamSet(Params({ a: '1', b: '2' }))
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
    const a = ParamSet({ foo: ['a', 'b'] });
    const b = ParamSet({ foo: ['c'] });
    addParamSet(a, b);
    assert.deepEqual(a, ParamSet({ foo: ['a', 'b', 'c'] }));
  });

  it('adds one param to empty params', () => {
    const a = ParamSet({});
    const b = ParamSet({ foo: ['c'] });
    addParamSet(a, b);
    assert.deepEqual(a, ParamSet({ foo: ['c'] }));
  });

  it('adds empty params to set of one params', () => {
    const a = ParamSet({ foo: ['c'] });
    const b = ParamSet({});
    addParamSet(a, b);
    assert.deepEqual(a, ParamSet({ foo: ['c'] }));
  });

  it('adds params for disjoint key sets', () => {
    const a = ParamSet({ foo: ['c'] });
    const b = ParamSet({ bar: ['b'] });
    addParamSet(a, b);
    assert.deepEqual(a, ParamSet({ foo: ['c'], bar: ['b'] }));
  });
});
