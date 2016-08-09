describe('Test sk.string functions.',
  function() {
    function testSortStrings() {
      var test = function(input, expected) {
        assert.deepEqual(sk.sortStrings(input), expected);
      }
      test([], []);
      test([''], ['']);
      test(['A', 'b', 'C', 'f', 'E', 'd'], ['A', 'b', 'C', 'd', 'E', 'f']);
      test(['AAA', 'a', '', 'aaaa', 'AA'], ['', 'a', 'AA', 'AAA', 'aaaa']);
      test(['5', '30', '100', '1'], ['1', '100', '30', '5']);
      test(['Nut', 'nOt', 'NIt', 'neT', 'NaT', 'nOTe', 'NATe'],
           ['NaT', 'NATe', 'neT', 'NIt', 'nOt', 'nOTe', 'Nut']);
    }

    it('sk.sortStrings sorts ignoring case', testSortStrings);

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
