/**
 * @module common-sk/modules/confirm-dialog-sk
 * @description <h2><code>confirm-dialog-sk</code></h2>
 *
 * <p>
 * This element pops up a dialog with OK and Cancel buttons. Its open method returns a Promise
 * which will resolve when the user clicks OK or reject when the user clicks Cancel.
 * </p>
 *
 * @example
 *
 * <confirm-dialog-sk id="confirm_dialog"></confirm-dialog-sk>
 *
 * <script>
 *   (function(){
 *     $('#confirm-dialog').open("Proceed with taking over the world?").then(() => {
 *       // Do some thing on confirm.
 *     }).catch(() => {
 *       // Do some thing on cancel.
 *     });
 *   })();
 * </script>
 *
 */
import { html, render } from 'lit-html/lib/lit-extended'

import { upgradeProperty } from 'elements-sk/upgradeProperty'

import 'elements-sk/dialog-sk'
import 'elements-sk/buttons'

const template = (ele) => html`<dialog-sk>
  <h2>Confirm</h2>
  <div class=message>${ele._message}</div>
  <div class=buttons>
  <button on-click=${e => ele._dismiss()}>Cancel</button>
  <button on-click=${e => ele._confirm()}>OK</button>
  </div>
</dialog-sk>`;

window.customElements.define('confirm-dialog-sk', class extends HTMLElement {
  constructor() {
    super();
    this._resolve = null;
    this._reject = null;
    this._message = '';
  }

  connectedCallback() {
    this._render();
  }

  /**
   * Display the dialog.
   *
   * @param {string} message - Message to display. Text only, any markup will be escaped.
   * @returns {Promise} Returns a Promise that resolves on OK, and rejects on Cancel.
   *
   */
  open(message) {
    this._message = message;
    this._render();
    this.firstChild.shown = true;
    return new Promise((resolve, reject) => {
      this._resolve = resolve;
      this._reject = reject;
    });
  }

  _dismiss() {
    this.firstChild.shown = false;
    this._reject();
  }

  _confirm() {
    this.firstChild.shown = false;
    this._resolve();
  }

  _render() {
    render(template(this), this);
  }
});
