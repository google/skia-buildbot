describe('sk.object functions',
  function() {
    function testGetDelta() {
      var test = function(o, d, expected) {
        assert.deepEqual(sk.object.getDelta(o, d), expected);
      }
      test({}, {}, {});
      test({a: "foo"}, {a: "foo"}, {});
      var first = {};  // Ensure getDelta does not modify its arguments.
      test(first, {a: "foo"}, {});
      assert.deepEqual(first, {});
      var second = {};  // Ensure getDelta does not modify its arguments.
      test({a: "foo"}, second, {a: "foo"});
      assert.deepEqual(second, {});
      test({a: "foo"}, {a: "bar"}, {a: "foo"});
      test({a: "foo", b: "bar"}, {a: true, c: "bar"}, {a: "foo", b: "bar"});
      test(["one", 2, 3.0], [1, "2", 3], {'0': "one"});
      test({a: undefined, b: NaN, c: null}, {a: true, b: true, c: true},
           {a: undefined, b: NaN, c: null});
      test({a: undefined, b: NaN, c: false}, {a: null, b: null, c: null},
           {b: NaN, c: false});
      var noop = function () {};
      test({a: test, b: noop}, {a: test, b: function () {}}, {b: noop});
    }

    function testApplyDelta() {
      var test = function(delta, o, expected) {
        assert.deepEqual(sk.object.applyDelta(delta, o), expected);
      }
      test({}, {}, {});
      test({}, {a: "foo"}, {a: "foo"});
      var first = {a: "bar"};  // Ensure applyDelta does not modify its arguments.
      test(first, {a: "foo"}, {a: "bar"});
      assert.deepEqual(first, {a: "bar"});
      var second = {a: "bar"};  // Ensure applyDelta does not modify its arguments.
      test({a: "foo"}, second, {a: "foo"});
      assert.deepEqual(second, {a: "bar"});
      test({a: "foo"}, {a: "bar", b: "baz"}, {a: "foo", b: "baz"});
      test({a: "foo", b: "baz"}, {a: "bar"}, {a: "foo"});
      test({a: "foo", b: "bar"}, {a: true, c: "bar"},
           {a: "foo", c: "bar"});
      test(["one"], [1, "2", 3], {'0': "one", '1': "2", '2': 3});
      test({b: NaN, c: false}, {a: null, b: null, c: null},
           {a: null, b: NaN, c: false});
    }

    it('should be able get differences and apply differences', function() {
      testGetDelta();
      testApplyDelta();
    });
  }
);
