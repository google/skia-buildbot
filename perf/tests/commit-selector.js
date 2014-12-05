describe('<commit-selector-sk>', function () {
   before(function(done) {
        var l = document.createElement('link');
        l.rel = 'import';
        l.href = 'base/res/imp/commit-selector.html';
        document.head.appendChild(l);
        l.onload = function() {
          done();
        };
    });

   it('generates events on selection', function (done) {
     var ele = document.createElement('commit-selector-sk');
     assert.equal(ele.currentCommitMsg, 'No commit chosen');

     ele.addEventListener('change', function(e) {
       assert.equal(e.detail.hash, 'foo');
       done();
     });

     var detail = {
       item: {
         dataset: {
           label: 'foo'
         }
       }
     };
     ele.$.selector.dispatchEvent(new CustomEvent('core-activate', {detail: detail}));
   });
});
