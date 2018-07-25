import { ParamSet } from './index.js'

describe('ParamSet',
  function() {
    function testParamSet() {
      let ps = new ParamSet();
      let p = {
        "foo": "1",
        "bar": "a",
      };
      ps.add(p);
      assert.isTrue(ps.match(p));
      assert.isTrue(ps.match({}));
    }

    it('should be able get match against params', function() {
      testParamSet();
    });
  }
);
