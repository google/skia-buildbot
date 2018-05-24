import './index.js'

import { $$, DomReady } from 'common-sk/modules/dom';

// Can't use import fetch-mock because the library isn't quite set up
// correctly for it, and we get strange errors about "this" not being defined.
const fetchMock = require('fetch-mock')

DomReady.then(() => {
  const ele = $$('#ele');
  ele.job='Debug-GPU';
  ele.commit='abc123f';

  let src = $$('iframe').src;
  if (src !== window.location.origin + '/cov_html/abc123f/Debug-GPU/html/index.html') {
    throw 'Error, src was wrong - should not have been ' + src
  }

  console.log('Iframe was working well');
});
