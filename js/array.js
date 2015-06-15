describe('Test sk.array functions.',
  function() {
    function testEqual() {
      assert.isTrue(sk.array.equal([], []));
      assert.isFalse(sk.array.equal([], ['1']));
      assert.isFalse(sk.array.equal(['1'], []));
      assert.isTrue(sk.array.equal(['1'], ['1']));
      assert.isFalse(sk.array.equal(['1', '2'], ['1']));
      assert.isFalse(sk.array.equal(['1'], ['1', '2']));
      assert.isTrue(sk.array.equal(['1', '2'], ['1', '2']));
    }

    it('should be able to query for elements', function() {
      testEqual();
    });
  }
);
