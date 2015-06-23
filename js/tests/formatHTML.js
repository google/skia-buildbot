describe('sk.formatHTML',
  function() {
    function testFormatHTML(linksInNewWindow) {
      var longLink =
          "https://docs.google.com/a/google.com/document/d/15uknTgFlGOZY7mhFH8cM5FtRfcR6SvA-UZp61IBhBWU/edit?usp=sharing";
      var newWindow = linksInNewWindow ? ' target="_blank"' : '';
      var testCases = [
        ["", ""],
        ["This string does not require formatting.",
         "This string does not require formatting."],
        ["\nI've got some newlines.\nTurn them into BRs.\n",
         "<br/>I've got some newlines.<br/>Turn them into BRs.<br/>"],
        ["\rI'm an old Mac...\r...with newlines.\r",
         "<br/>I'm an old Mac...<br/>...with newlines.<br/>"],
        ["\r\nI'm Windows\r\nLook at all my newlines!\r\n",
         "<br/>I'm Windows<br/>Look at all my newlines!<br/>"],
        ["Just type into http://www.google.com/ to search.",
         'Just type into <a href="http://www.google.com/"' + newWindow +
             '>http://www.google.com/</a> to search.'],
        ["All my passwords are stored here: " + longLink,
         'All my passwords are stored here: <a href="' + longLink + '"' +
             newWindow + '>' + longLink + '</a>'],
        ["Bugs are linkified like skia:123",
         'Bugs are linkified like ' +
             '<a href="http://skbug.com/123" target="_blank">skia:123</a>']
      ];
      for (var testCase of testCases) {
        assert.equal(sk.formatHTML(testCase[0], linksInNewWindow), testCase[1]);
      }
    }

    it('should format plain text as HTML', function() {
      testFormatHTML(false);
      testFormatHTML(true);
    });
  }
);
