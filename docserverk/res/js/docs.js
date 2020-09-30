(() => {
  // Open the side drawer with the navigation menu.
  document.querySelector('#menu').addEventListener('click', (e) => {
    document.querySelector('#drawer').classList.toggle('opened');
    e.stopPropagation();
  });

  // Close the side drawer.
  document.querySelector('body').addEventListener('click', () => {
    document.querySelector('#drawer').classList.remove('opened');
  });

  // highlightNav highlights where we are in the navigation.
  const highlightNav = () => {
    // PR is the Preffify global namespace.
    if (PR && PR.prettyPrint) {
      PR.prettyPrint();
    }
    document.querySelectorAll('#drawer li a').forEach((e) => {
      if (e.dataset.path === window.location.pathname) {
        e.classList.add('selected');
        document.querySelector('title').innerText = e.innerText;
      } else {
        e.classList.remove('selected');
      }
    });
  };

  // Shortcut the links and handle them via XHR, that way we only
  // pay the loading time once, yet still retain full URLs.
  document.querySelectorAll('#drawer li a').forEach((link) => {
    link.addEventListener('click', (e) => {
      // Preserve query parameters as we navigate.
      const q = window.location.search;
      let url = e.target.dataset.path;
      if (q !== '') {
        url += q;
      }
      fetch(`/_${url}`)
        .then((resp) => {
          if (resp.ok) {
            return resp.text();
          }
          throw new Error('Failed to load.');
        })
        .then((content) => {
          if (content.indexOf('<script') !== -1) {
            // We can't render <script> using innerHTML, so we just go
            // directly to the page.
            // https://developer.mozilla.org/en-US/docs/Web/API/Element/innerHTML#Security_considerations
            window.location.href = url;
          } else {
            window.history.pushState(null, null, url);
            highlightNav();
            document.querySelector('main').innerHTML = content;
            window.scrollTo(0, 0);
          }
        });
      e.preventDefault();
    });
  });

  highlightNav();

  // Turn on search.
  const cx = '009791159600898516779:8-nlv0iznho';
  const gcse = document.createElement('script');
  gcse.type = 'text/javascript';
  gcse.async = true;
  gcse.src = `${
    document.location.protocol === 'https:' ? 'https:' : 'http:'
  }//cse.google.com/cse.js?cx=${cx}`;
  const s = document.getElementsByTagName('script')[0];
  s.parentNode.insertBefore(gcse, s);
})();
