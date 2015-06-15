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
        assert.equal(sk.toCapWords(input), tc[input]);
      }
    }

    it('Verify that sk.toCapWords works as expected.', function() {
      testCapWords();
    });
  }
);
