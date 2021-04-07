/**
 * @module modules/alogin-sk
 * @description <h2><code>alogin-sk</code></h2>
 *
 * Handles logging into applications that use alogin.Login.
 *
 * @attr {boolean} testing_offline - If we should really fetch Login status or
 *  use mock data.
 */
import { define } from 'elements-sk/define';
import { errorMessage } from 'elements-sk/errorMessage';
import { html } from 'lit-html';
import { ElementSk } from '../ElementSk';
import { alogin } from '../json';

/**
 * Returns a Promise that resolves when we have received the login status, and
 * rejects if there was an error retrieving the login status.
 */
const loggedIn = async (): Promise<alogin.Status> => {
  const resp = await fetch('/_/login/status');
  if (!resp.ok) {
    errorMessage(`Failed to load login status: ${resp.statusText}`);
    return defaultStatus;
  }
  return resp.json();
};

const defaultStatus: alogin.Status = {
  email: '',
  login: '',
  logout: '',
};

const fakeStatus: alogin.Status = {
  email: 'test@example.com',
  login: '/login/',
  logout: '/logout/',
};

export class AloginSk extends ElementSk {
  /**
   * A promise that resolves to the users logged in status.
   *
   * The value of this may be altered by the 'testing_offline' attribute.
   */
  statusPromise: Promise<alogin.Status> = loggedIn();

  private status: alogin.Status = defaultStatus;

  constructor() {
    super(AloginSk.template);
  }

  private static template = (ele: AloginSk) => html`
  <span class=email>
    ${ele.status.email}
  </span>
  <a
    class=logInOut
    href="${ele.status.email ? ele.status.logout : ele.status.login}"
  >
    ${ele.status.email ? 'Login' : 'Logout'}
  </a>
  `;

  async connectedCallback(): Promise<void> {
    super.connectedCallback();

    if (this.hasAttribute('testing_offline')) {
      this.statusPromise = Promise.resolve(fakeStatus);
      this.status = fakeStatus;
      this._render();
      return;
    }
    this.status = await this.statusPromise;
    this._render();
  }
}

define('alogin-sk', AloginSk);
