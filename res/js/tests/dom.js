describe('Test DOM-manipulation functions.',
  function() {
    function testClearChildren() {
      var testCases = [
        "",
        "Text",
        "Text <p>para</p> text",
        "<span>span</span> <div>inner div</div>"
      ];
      for (var testCase in testCases) {
        var div = document.createElement('div');
        div.innerHTML = testCase;
        document.body.appendChild(div);
        sk.clearChildren(div);
        assert.equal(div.innerHTML, "");
      }
    }

    it('verifies that clearChildren emties the element', function() {
      testClearChildren();
    });

    function testFindParent() {
      // Add an HTML tree to the document.
      var div = document.createElement('div');
      div.innerHTML =
          '<div id=a>' +
          '  <p id=aa>' +
          '    <span id=aaa>span</span>' +
          '    <span id=aab>span</span>' +
          '  </p>' +
          '  <span id=ab>' +
          '    <p id=aba>para</p>' +
          '  </span>' +
          '  <div id=ac>' +
          '    <p id=aca>para</p>' +
          '  </div>' +
          '</div>' +
          '<div id=b>' +
          '  <p id=ba>para</p>' +
          '</div>' +
          '<span id=c>' +
          '  <span id=ca>' +
          '    <p id=caa>para</p>' +
          '  </span>' +
          '</span>';
      assert.equal(sk.findParent($$$('#a', div), 'DIV'), $$$('#a', div), 'Top level');
      assert.equal(sk.findParent($$$('#a', div), 'SPAN'), null);
      assert.equal(sk.findParent($$$('#aa', div), 'DIV'), $$$('#a', div));
      assert.equal(sk.findParent($$$('#aaa', div), 'DIV'), $$$('#a', div));
      assert.equal(sk.findParent($$$('#aaa', div), 'P'), $$$('#aa', div));
      assert.equal(sk.findParent($$$('#aab', div), 'SPAN'), $$$('#aab', div));
      assert.equal(sk.findParent($$$('#ab', div), 'P'), null);
      assert.equal(sk.findParent($$$('#aba', div), 'SPAN'), $$$('#ab', div));
      assert.equal(sk.findParent($$$('#ac', div), 'DIV'), $$$('#ac', div));
      assert.equal(sk.findParent($$$('#aca', div), 'DIV'), $$$('#ac', div));
      assert.equal(sk.findParent($$$('#ba', div), 'DIV'), $$$('#b', div));
      assert.equal(sk.findParent($$$('#caa', div), 'DIV'), div);
      assert.equal(sk.findParent($$$('#ca', div), 'SPAN'), $$$('#ca', div));
      assert.equal(sk.findParent($$$('#caa', div), 'SPAN'), $$$('#ca', div));
    }

    it('verifies that findParent identifies the correct element', function() {
      testFindParent();
    });
  }
);
