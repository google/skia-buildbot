describe('Test sk.linkifyBugs.',
  function() {
    function testBugReferences() {
      var string_to_expected = {
        '': '',
        '<a href="google.com">existing link</a>': '<a href="google.com">existing link</a>',
        'test skia:123 ': 'test <a href="http://skbug.com/123" target="_blank">skia:123</a> ',
        'test chromium:123 ': 'test <a href="http://crbug.com/123" target="_blank">chromium:123</a> ',
        'skia:123 chromium:123': '<a href="http://skbug.com/123" target="_blank">skia:123</a> <a href="http://crbug.com/123" target="_blank">chromium:123</a>',
        ' nonexistant:123 ': ' nonexistant:123 ',
      }
      for (var s in string_to_expected) {
        assert.equal(sk.linkifyBugs(s), string_to_expected[s])
      }
    }

    it('bug references should be substituted for links', function() {
      testBugReferences();
    });
  }
);
