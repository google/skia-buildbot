describe('Test sk.string functions.',
  function() {
    function testCapWords() {
      var tc = {
        "": "",
        "a": "A",
        "A": "A",
        "a ": "A ",
        "abc": "Abc",
        "aBc": "ABc",
        "abc def": "Abc Def",
      };
      for (var input in tc) {
        assert.equal(input.toCapWords(), tc[input]);
      }
    }

    it('Verify that String.prototype.toCapWords works as expected.', function() {
      testCapWords();
    });
  }
);
