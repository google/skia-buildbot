describe('sk.key functions',
  function() {
    function testMatches() {
      var test = function(k, pn, pv, expected) {
        assert.equal(sk.key.matches(k, pn, pv), expected);
      }
      test(',config=565,', 'config', '565', true);
      test(',config=8888,', 'config', '565', false);
      test(',foo=565,', 'config', '565', false);
    }

    function testParses() {
      var test = function(k, expected) {
        assert.deepEqual(sk.key.toObject(k), expected);
      }
      test(',config=565,', {config: '565'});
      test('config=,', {});
      test(',config=565,arch=x86', {config: '565', arch: 'x86'});
      test('', {});
      test(',', {});
      test(',,', {});
    }

    it('should match and parse structured keys.', function() {
      testMatches();
      testParses();
    });
  }
);
