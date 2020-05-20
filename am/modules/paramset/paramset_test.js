import * as paramset from './index';

describe('ParamSet',
  () => {
    function testParamSet() {
      const ps = {};
      // The empty paramset matches everything.
      assert.isTrue(paramset.match(ps, {}));
      assert.isTrue(paramset.match(ps, { foo: '2', bar: 'a' }));

      const p = {
        foo: '1',
        bar: 'a',
      };
      paramset.add(ps, p);
      assert.isTrue(paramset.match(ps, p));
      assert.isFalse(paramset.match(ps, {}));

      paramset.add(ps, {
        foo: '2',
        bar: 'b',
      });

      paramset.add(ps, {
        foo: '1',
        bar: 'b',
      });

      assert.isTrue(paramset.match(ps, { foo: '2', bar: 'a' }));
      assert.isTrue(paramset.match(ps, { foo: '2', bar: 'a', baz: 'other' }));
      assert.isFalse(paramset.match(ps, { bar: 'a' }));
      assert.isFalse(paramset.match(ps, { foo: '2' }));
      assert.isFalse(paramset.match(ps, { }));
      assert.isFalse(paramset.match(ps, { foo: '3', bar: 'a' }));
      assert.isFalse(paramset.match(ps, { foo: '2', bar: 'c' }));
    }

    function testParamSetWithIgnore() {
      const ps = {};
      const p = {
        foo: '1',
        bar: 'a',
        description: 'long rambling text',
      };
      paramset.add(ps, p, ['description']);
      assert.isTrue(paramset.match(ps, p));
      assert.isTrue(paramset.match(ps, {
        foo: '1',
        bar: 'a',
      }));
      assert.isFalse(paramset.match(ps, {}));
    }

    function testParamSetWithRegex() {
      const ps = {};
      paramset.add(ps, { foo: '2.*' });
      assert.isTrue(paramset.match(ps, { foo: '2345', bar: 'aa' }));
      assert.isFalse(paramset.match(ps, { foo: '345', bar: 'bb' }));
      paramset.add(ps, { bar: '.*a' });
      assert.isTrue(paramset.match(ps, { foo: '2345', bar: 'aa' }));
      assert.isFalse(paramset.match(ps, { foo: '2345', bar: 'bb' }));

      // Test with paramset with both regex and non-regex by adding another
      // value to existing key.
      paramset.add(ps, { foo: 'blah' });
      assert.isTrue(paramset.match(ps, { foo: '2345', bar: 'aa' }));
      assert.isTrue(paramset.match(ps, { foo: 'blah', bar: 'aa' }));
      assert.isFalse(paramset.match(ps, { foo: '345', bar: 'aa' }));
      assert.isFalse(paramset.match(ps, { foo: '2345', bar: 'bb' }));
    }

    it('should be able get match against params', () => {
      testParamSet();
      testParamSetWithIgnore();
      testParamSetWithRegex();
    });
  });
