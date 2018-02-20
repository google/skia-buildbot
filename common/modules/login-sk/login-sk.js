import { errorMessage } from 'common/errorMessage';
import { login } from 'common/login';

//  The <login-sk> custom element.
//
//  Use the Login promise to display the current login status and provides
//  login/logout links. Reports errors via errorMessage.
//
//  Properties:
//    None.
//
//  Methods:
//    None.
//
//  Events:
//    None. But error-sk will be sent from document on a network error.
window.customElements.define('login-sk', class extends HTMLElement {
  connectedCallback() {
    this.innerHTML = `<span class=email>Loading...</span><a class=logInOut></a>`;
    login.then(status => {
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
});
