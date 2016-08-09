describe('Test Importer.',
  function() {
    var container;

    afterEach(function () {
      if (container) { document.body.removeChild(container); }
    });

    function testImporter() {
      var pending = new Promise(function (resolve, reject) {
        var link = document.createElement('link');
        link.rel = 'import';
        link.href = '/base/tests/import.html'
        link.onload = resolve;
        link.onerror = function (e) { reject(new Error(e)); };
        document.head.appendChild(link);
      }).then(function () {
        container = document.createElement('div');
        sk.addWhammo(container);
        document.body.appendChild(container);
        assert.equal(container.innerHTML, '<p class="whammo">whammo para</p>');
      })
      return pending;
    }

    it('should import templates', testImporter);
  }
);
