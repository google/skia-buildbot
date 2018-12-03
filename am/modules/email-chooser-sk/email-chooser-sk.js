/**
 * @module email-chooser-sk
 * @description <h2><code>email-chooser-sk</code></h2>
 *
 * <p>
 * This element pops up a dialog with OK and Cancel buttons. Its open method returns a Promise
 * which will resolve when the user clicks OK after selecting
 * an email or reject when the user clicks Cancel.
 * </p>
 *
 */
import { html, render } from 'lit-html'
import { $$ } from 'common-sk/modules/dom'

import 'elements-sk/dialog-sk'
import 'elements-sk/styles/buttons'
import 'elements-sk/styles/select'

const template = (ele) => html`<dialog-sk>
  <h2>Assign</h2>
  <select size=10 @input=${ele._input}>
    <option value='' selected>(un-assign)</option>
    ${ele._emails.map(email => html`<option value=${email}>${email}</option>`)}
  </select>
  <div class=buttons>
    <button @click=${ele._dismiss}>Cancel</button>
    <button @click=${ele._confirm}>OK</button>
  </div>
</dialog-sk>`;

window.customElements.define('email-chooser-sk', class extends HTMLElement {
  constructor() {
    super();
    this._resolve = null;
    this._reject = null;
    this._emails = [];
    this._selected = '';
  }

  connectedCallback() {
    this._render();
    this._dialog = $$('dialog-sk');
  }

  /**
   * Display the dialog.
   *
   * @param emails {Array} List of emails to choose from.
   * @returns {Promise} Returns a Promise that resolves on OK, and rejects on Cancel.
   *
   */
  open(emails) {
    this._emails = emails;
    this._render();
    this._dialog.shown = true;
    $$('select', this).focus();
    return new Promise((resolve, reject) => {
      this._resolve = resolve;
      this._reject = reject;
    });
  }

  _input(e) {
    this._selected = e.srcElement.value;
  }

  _dismiss() {
    this._dialog.shown = false;
    this._reject();
  }

  _confirm() {
    this._dialog.shown = false;
    this._resolve(this._selected);
  }

  _render() {
    render(template(this), this, {eventContext: this});
  }
});
