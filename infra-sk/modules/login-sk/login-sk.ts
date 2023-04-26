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
 * @attr {string} login_host - If it needs to be something other than skia.org.
 *  If login_host is not specified then skia.org will be used.
 *
 * <p>
 */
import { define } from '../../../elements-sk/modules/define';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';
import { baseDomain, LoginTo } from '../login';

define(
  'login-sk',
  class extends HTMLElement {
    connectedCallback() {
      this.innerHTML =
        '<span class=email>Loading...</span><a class=logInOut></a>';

      const host = baseDomain();

      const login = `https://${host}/login/`;
      const logout = `https://${host}/logout/`;

      if (this.testingOffline) {
        this.querySelector<HTMLSpanElement>('.email')!.textContent =
          'test@example.com';
        const logInOut = this.querySelector<HTMLAnchorElement>('.logInOut')!;
        logInOut.href = `https://${host}/logout/?redirect=${encodeURIComponent(
          document.location.toString()
        )}`;
        logInOut.textContent = 'Logout';
      } else {
        LoginTo(`/loginstatus/`)
          .then((status) => {
            this.querySelector<HTMLSpanElement>('.email')!.textContent =
              status.Email;
            const logInOut =
              this.querySelector<HTMLAnchorElement>('.logInOut')!;
            if (!status.Email) {
              logInOut.href = login;
              logInOut.textContent = 'Login';
            } else {
              logInOut.href = `${logout}?redirect=${encodeURIComponent(
                document.location.toString()
              )}`;
              logInOut.textContent = 'Logout';
            }
          })
          .catch(errorMessage);
      }
    }

    /** Reflects the testing_offline attribute for ease of use. */
    get testingOffline(): boolean {
      return this.hasAttribute('testing_offline');
    }

    set testingOffline(val: boolean) {
      if (val) {
        this.setAttribute('testing_offline', '');
      } else {
        this.removeAttribute('testing_offline');
      }
    }

    /** Which host should be used for login URLs. */
    get loginHost(): string {
      return this.getAttribute('login_host') || '';
    }

    set loginHost(val: string) {
      if (val) {
        this.setAttribute('login_host', val);
      }
    }
  }
);
