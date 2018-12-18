sk.DomReady.then(function() {
  prettyPrint();

  // Open the side drawer with the navigation menu.
  $$$('button').addEventListener('click', function(e) {
    $$$('#drawer').classList.add('opened');
    e.stopPropagation();
  });

  // Close the side drawer.
  $$$('body').addEventListener('click', function() {
    $$$('#drawer').classList.remove('opened');
  });

  // highlightNav highlights where we are in the navigation.
  var highlightNav = function() {
    $$('#drawer li a').forEach(function(e) {
      if (e.dataset.path == window.location.pathname) {
        e.classList.add('selected');
        $$$('title').innerText = e.innerText;
      } else {
        e.classList.remove('selected');
      }
    });
  }

  // Shortcut the links and handle them via XHR, that way we only
  // pay the loading time once, yet still retain full URLs.
  $$('#drawer li a').forEach(function(e) {
    e.addEventListener('click', function(e) {
      // Preserve query parameters as we navigate.
      var q = window.location.search;
      var url = e.target.dataset.path;
      if (q != "") {
        url += q;
      }
      sk.get('/_'+url).then(function(content) {
        if (content.indexOf('<script') !== -1) {
          // We can't render <script> using innerHTML, so we just go
          // directly to the page.
          // https://developer.mozilla.org/en-US/docs/Web/API/Element/innerHTML#Security_considerations
          window.location.href = url;
        } else {
          window.history.pushState(null, null, url);
          highlightNav();
          $$$('main').innerHTML = content;
          window.scrollTo(0,0);
          prettyPrint();
        }
      });
      e.preventDefault();
    });
  });

  highlightNav();
});

(function() {
  var cx = '009791159600898516779:8-nlv0iznho';
  var gcse = document.createElement('script');
  gcse.type = 'text/javascript';
  gcse.async = true;
  gcse.src = (document.location.protocol == 'https:' ? 'https:' : 'http:') +
    '//cse.google.com/cse.js?cx=' + cx;
  var s = document.getElementsByTagName('script')[0];
  s.parentNode.insertBefore(gcse, s);
})();
