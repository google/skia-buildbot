/** @module common/login-sk */
import { errorMessage } from 'common/errorMessage';
import { Login } from 'common/login';

/** <code>login-sk</code>
 *
 * <p>
 * The <login-sk> custom element. Uses the Login promise to display the
 * current login status and provides login/logout links. Reports errors via
 * {@linkcode module:common/errorMessage}.
 * </p>
 */
class LoginSk extends HTMLElement {
  connectedCallback() {
    this.innerHTML = `<span class=email>Loading...</span><a class=logInOut></a>`;
    Login.then(status => {
      this.querySelector('.email').textContent = status.Email;
      let logInOut = this.querySelector('.logInOut');
      if (!status.Email) {
          logInOut.href = status.LoginURL;
          logInOut.textContent = 'Login';
      } else {
          logInOut.href = "/logout/?redirect=" + encodeURIComponent(document.location);
          logInOut.textContent = 'Logout';
      }
    }).catch(errorMessage);
  }
}

window.customElements.define('login-sk', LoginSk);
