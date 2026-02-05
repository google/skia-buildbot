/**
 * @module modules/alogin-sk
 * @description <h2><code>alogin-sk</code></h2>
 *
 * Handles logging into applications that use alogin.Login.
 *
 * @attr {boolean} testing_offline - If we should really fetch Login status or
 *  use mock data.
 *
 * @attr {string} url - The url that returns the JSON serialized alogin.Status response.
 */
import { html, LitElement } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';
import { Status } from '../json';
import { rootDomain } from '../url';

export const defaultStatusURL = '/_/login/status';

let loggedInPromise: Promise<Status> | null = null;

/**
 * Resets the loggedInPromise cache. Should only be used for testing.
 */
export const resetLoggedInPromise = (): void => {
  loggedInPromise = null;
};

/**
 * Returns a Promise that resolves when we have received the login status, and
 * rejects if there was an error retrieving the login status. The result is
 * cached, so this will only result in a single fetch call.
 */
export const LoggedIn = (url: string = defaultStatusURL): Promise<Status> => {
  if (loggedInPromise) {
    return loggedInPromise;
  }
  loggedInPromise = (async () => {
    try {
      const resp = await fetch(url);
      if (!resp.ok) {
        await errorMessage(`Failed to load login status: ${resp.statusText}`);
        loggedInPromise = null; // Clear cache on error to allow retries.
        return defaultStatus;
      }
      return await resp.json();
    } catch (e) {
      loggedInPromise = null; // Clear cache on error to allow retries.
      throw e;
    }
  })();
  return loggedInPromise;
};

const defaultStatus: Status = {
  email: '',
  roles: [],
};

const fakeStatus: Status = {
  email: 'test@example.com',
  roles: ['viewer'],
};

@customElement('alogin-sk')
export class AloginSk extends LitElement {
  /**
   * A promise that resolves to the users logged in status.
   *
   * The value of this may be altered by the 'testing_offline' attribute.
   */
  statusPromise: Promise<Status> = Promise.resolve(defaultStatus);

  @property({ type: Boolean, attribute: 'testing_offline' })
  testingOffline: boolean = false;

  @property({ type: String })
  url: string = '';

  private login: string = '';

  private logout: string = '';

  @state()
  private status: Status = defaultStatus;

  createRenderRoot() {
    return this;
  }

  async connectedCallback(): Promise<void> {
    super.connectedCallback();

    const domain = rootDomain();

    this.login = `https://${domain}/login/`;
    this.logout = `https://${domain}/logout/`;

    if (this.testingOffline) {
      this.statusPromise = Promise.resolve(fakeStatus);
      this.status = fakeStatus;
    } else {
      this.statusPromise = LoggedIn(this.url || undefined);
      this.status = await this.statusPromise;
    }
  }

  render() {
    return html`
      <span class="email"> ${this.status.email} </span>
      <a class="logInOut" href="${this.status.email ? this.logout : this.login}">
        ${this.status.email ? 'Logout' : 'Login'}
      </a>
    `;
  }
}
