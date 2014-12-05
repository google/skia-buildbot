'use strict';
mocha.setup('bdd');
(function() {
  window.__karma__.loaded = function() {
    window.addEventListener('polymer-ready', function() {
      window.__karma__.start();
    });
  };

  var l = document.createElement('link');
  l.rel = 'import';
  l.href = 'base/res/vul/elements.html';
  document.head.appendChild(l);
})();
