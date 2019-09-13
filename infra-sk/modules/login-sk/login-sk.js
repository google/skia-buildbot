/**
 * @module infra-sk/modules/login-sk
 * @description <h2><code>login-sk</code></h2>
 *
 * <p>
 * The <login-sk> custom element. Uses the Login promise to display the
 * current login status and provides login/logout links. Reports errors via
 * {@linkcode module:elements-sk/error-toast-sk}.
 * </p>
 *
 * @attr {boolean} testing_offline - If we should really poll for Login status
 *  or use mock data.
 *
 * <p>
 */
import { define } from 'elements-sk/define'
import { errorMessage } from 'elements-sk/errorMessage';
import { Login } from '../login';

define('login-sk', class extends HTMLElement {
  connectedCallback() {
    this.innerHTML = `<span class=email>Loading...</span><a class=logInOut></a>`;
    if (this.testingOffline) {
      this.querySelector('.email').textContent = "test@example.com";
      const logInOut = this.querySelector('.logInOut');
      logInOut.href = 'https://skia.org/logout/?redirect=' + encodeURIComponent(document.location);
      logInOut.textContent = 'Logout';
    } else {
      Login.then((status) => {
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
  }

  /** @prop testingOffline {boolean} Reflects the testing_offline attribute for ease of use.
   */
  get testingOffline() { return this.hasAttribute('testing_offline'); }
  set testingOffline(val) {
    if (val) {
      this.setAttribute('testing_offline', '');
    } else {
      this.removeAttribute('testing_offline');
    }
  }
});
