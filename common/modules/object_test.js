import * as object from './object.js'

describe('object functions',
  function() {
    function testGetDelta() {
      let test = function(o, d, expected) {
        assert.deepEqual(object.getDelta(o, d), expected);
      }
      test({}, {}, {});
      test({a: "foo"}, {a: "foo"}, {});
      let first = {};  // Ensure getDelta does not modify its arguments.
      test(first, {a: "foo"}, {});
      assert.deepEqual(first, {});
      let second = {};  // Ensure getDelta does not modify its arguments.
      test({a: "foo"}, second, {a: "foo"});
      assert.deepEqual(second, {});
      test({a: "foo"}, {a: "bar"}, {a: "foo"});
      test({a: "foo", b: "bar"}, {a: true, c: "bar"}, {a: "foo", b: "bar"});
      test(["one", 2, 3.0], [1, "2", 3], {'0': "one", '1': 2});
      test({a: undefined, b: NaN, c: null}, {a: true, b: true, c: true},
           {a: undefined, b: NaN, c: null});
      test({a: undefined, b: NaN, c: false}, {a: null, b: null, c: null},
           {a: undefined, b: NaN, c: false});
    }

    function testApplyDelta() {
      let test = function(delta, o, expected) {
        assert.deepEqual(object.applyDelta(delta, o), expected);
      }
      test({}, {}, {});
      test({}, {a: "foo"}, {a: "foo"});
      let first = {a: "bar"};  // Ensure applyDelta does not modify its arguments.
      test(first, {a: "foo"}, {a: "bar"});
      assert.deepEqual(first, {a: "bar"});
      let second = {a: "bar"};  // Ensure applyDelta does not modify its arguments.
      test({a: "foo"}, second, {a: "foo"});
      assert.deepEqual(second, {a: "bar"});
      test({a: "foo"}, {a: "bar", b: "baz"}, {a: "foo", b: "baz"});
      test({a: "foo", b: "baz"}, {a: "bar"}, {a: "foo"});
      test({a: "foo", b: "bar"}, {a: true, c: "bar"},
           {a: "foo", c: "bar"});
      test(["one"], [1, "2", 3], {'0': "one", '1': "2", '2': 3});
      test({b: NaN, c: false}, {a: null, b: null, c: null},
           {a: null, b: null, c: false});
    }

    function testEquals() {
      assert.isTrue(object.equals(1, 1));
      assert.isTrue(object.equals([1,2], [1,2]));
      assert.isTrue(object.equals([], []));
      assert.isFalse(object.equals([1], []));
    }

    it('should be able get differences and apply differences', function() {
      testGetDelta();
      testApplyDelta();
      testEquals();
    });
  }
);
