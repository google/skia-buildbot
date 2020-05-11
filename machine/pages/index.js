import './index.scss';

import { errorMessage } from 'elements-sk/errorMessage';
import 'elements-sk/error-toast-sk';


// Super-simplistic polyfill for target=_inline proposal, that only does
// POST for forms of 'application/x-www-form-urlencoded'.
document.addEventListener('submit', async (e) => {
  // Only process forms when target=_inline.
  if (e.target.target !== '_inline') {
    return;
  }

  // Do the form processing ourselves.
  e.preventDefault();

  document.body.setAttribute('waiting', '');

  try {
  // Do a POST of the form values.
    const resp = await fetch(e.target.action, {
      method: 'POST',
      body: new FormData(e.target),
    });


    // Replace the innerHTML of the form with the HTML returned from the
    // server.
    e.target.innerHTML = await resp.text();
    document.body.removeAttribute('waiting');
  } catch (msg) {
    document.body.removeAttribute('waiting');
    errorMessage(msg);
  }
});
