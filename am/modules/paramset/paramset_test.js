import { ParamSet } from './index.js'

describe('ParamSet',
  function() {
    function testParamSet() {
      let ps = new ParamSet();
      // The empty paramset matches everything.
      assert.isTrue(ps.match({}));
      assert.isTrue(ps.match({"foo": "2", "bar": "a"}));

      let p = {
        "foo": "1",
        "bar": "a",
      };
      ps.add(p);
      assert.isTrue(ps.match(p));
      assert.isFalse(ps.match({}));

      ps.add({
        "foo": "2",
        "bar": "b",
      });

      ps.add({
        "foo": "1",
        "bar": "b",
      });

      assert.isTrue(ps.match( {"foo": "2", "bar": "a"}));
      assert.isTrue(ps.match( {"foo": "2", "bar": "a", "baz": "other"}));
      assert.isFalse(ps.match({            "bar": "a"}));
      assert.isFalse(ps.match({"foo": "2"            }));
      assert.isFalse(ps.match({                      }));
      assert.isFalse(ps.match({"foo": "3", "bar": "a"}));
      assert.isFalse(ps.match({"foo": "2", "bar": "c"}));
    }

    function testParamSetWithIgnore() {
      let ps = new ParamSet(['description']);
      // The empty paramset matches everything.
      assert.isTrue(ps.match({}));
      assert.isTrue(ps.match({"foo": "2", "bar": "a"}));

      let p = {
        "foo": "1",
        "bar": "a",
        "description": "long rambling text",
      };
      ps.add(p);
      assert.isTrue(ps.match(p));
      assert.isFalse(ps.match({}));

      ps.add({
        "foo": "2",
        "bar": "b",
        "description": "more long rambling text",
      });

      ps.add({
        "foo": "1",
        "bar": "b",
        "description": "and even more long rambling text",
      });

      assert.isTrue(ps.match( {"foo": "2", "bar": "a"}));
      assert.isTrue(ps.match( {"foo": "2", "bar": "a", "baz": "other"}));
      assert.isFalse(ps.match({            "bar": "a"}));
      assert.isFalse(ps.match({"foo": "2"            }));
      assert.isFalse(ps.match({                      }));
      assert.isFalse(ps.match({"foo": "3", "bar": "a"}));
      assert.isFalse(ps.match({"foo": "2", "bar": "c"}));
    }


    it('should be able get match against params', function() {
      testParamSet();
      testParamSetWithIgnore();
    });
  }
);
