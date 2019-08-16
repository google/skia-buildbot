/**
 * @module infra-sk/modules/login-sk
 * @description <h2><code>login-sk</code></h2>
 *
 * <p>
 * The <login-sk> custom element. Uses the Login promise to display the
 * current login status and provides login/logout links. Reports errors via
 * {@linkcode module:elements-sk/error-toast-sk}.
 * </p>
 */
import { define } from 'elements-sk/define'
import { errorMessage } from 'elements-sk/errorMessage';
import { Login } from '../login';

define('login-sk', class extends HTMLElement {
  connectedCallback() {
    this.innerHTML = `<span class=email>Loading...</span><a class=logInOut></a>`;
    Login.then(status => {
      this.querySelector('.email').textContent = status.Email;
      let logInOut = this.querySelector('.logInOut');
      if (!status.Email) {
          logInOut.href = status.LoginURL;
          logInOut.textContent = 'Login';
      } else {
          logInOut.href = 'https://skia.org/logout/?redirect=' + encodeURIComponent(document.location);
          logInOut.textContent = 'Logout';
      }
    }).catch(errorMessage);
  }
});
