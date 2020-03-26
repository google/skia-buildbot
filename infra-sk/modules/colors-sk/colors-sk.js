/**
 * @module infra-sk/modules/colors-sk
 * @description <h2><code>colors-sk</code></h2>
 *
 * <p>
 * The <colors-sk> custom element. Uses the Login promise to display the
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
import 'elements-sk/icon/invert-colors-icon-sk'

define('colors-sk', class extends HTMLElement {
  constructor() {
    super();
  }
  connectedCallback() {
    let icon = document.createElement('invert-colors-icon-sk');
    icon.addEventListener('click', (e) => document.body.classList.toggle('darkmode'));
    this.appendChild(icon);
    return;
  }
// TODO(weston): Add auto-accessibillity code that runs on connectedCallback.

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
