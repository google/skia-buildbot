describe('Test DOM-manipulation functions.',
  function() {
    afterEach(function() {
      document.body.innerHTML = "";
    });
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
      document.body.appendChild(div);
      assert.equal(sk.findParent($$$('#a'), 'DIV'), div);
      assert.equal(sk.findParent($$$('#a'), 'SPAN'), null);
      assert.equal(sk.findParent($$$('#aa'), 'DIV'), $$$('#a'));
      assert.equal(sk.findParent($$$('#aaa'), 'DIV'), $$$('#a'));
      assert.equal(sk.findParent($$$('#aaa'), 'P'), $$$('#aa'));
      assert.equal(sk.findParent($$$('#aab'), 'SPAN'), null);
      assert.equal(sk.findParent($$$('#ab'), 'P'), null);
      assert.equal(sk.findParent($$$('#aba'), 'SPAN'), $$$('#ab'));
      assert.equal(sk.findParent($$$('#ac'), 'DIV'), $$$('#a'));
      assert.equal(sk.findParent($$$('#aca'), 'DIV'), $$$('#ac'));
      assert.equal(sk.findParent($$$('#ba'), 'DIV'), $$$('#b'));
      assert.equal(sk.findParent($$$('#caa'), 'DIV'), div);
      assert.equal(sk.findParent($$$('#ca'), 'SPAN'), $$$('#c'));
      assert.equal(sk.findParent($$$('#caa'), 'SPAN'), $$$('#ca'));
    }

    it.only('verifies that findParent identifies the correct element', function() {
      testFindParent();
    });
  }
);
