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
import dialogPolyfill from 'dialog-polyfill';
import { define } from 'elements-sk/define';
import { html, render } from 'lit-html';
import { $$ } from 'common-sk/modules/dom';

import 'elements-sk/styles/buttons';
import 'elements-sk/styles/select';

function displayEmail(email, owner) {
  if (owner === email) {
    return html`<option value=${email}>${email} (alert owner)</option>`;
  }
  return html`<option value=${email}>${email}</option>`;
}

const template = (ele) => html`<dialog>
  <h2>Assign</h2>
  <select size=10 @input=${ele._input}>
    <option value='' selected>(un-assign)</option>
    ${ele._emails.map((email) => displayEmail(email, ele._owner))}
  </select>
  <div class=buttons>
    <button @click=${ele._dismiss}>Cancel</button>
    <button @click=${ele._confirm}>OK</button>
  </div>
</dialog>`;

define('email-chooser-sk', class extends HTMLElement {
  constructor() {
    super();
    this._resolve = null;
    this._reject = null;
    this._emails = [];
    this._owner = '';
    this._selected = '';
  }

  connectedCallback() {
    this._render();
    this._dialog = $$('dialog', this);
    dialogPolyfill.registerDialog(this._dialog);
  }

  /**
   * Display the dialog.
   *
   * @param emails {Array} List of emails to choose from.
   * @param owner {String} The owner of this incident if available. Optional.
   * @returns {Promise} Returns a Promise that resolves on OK, and rejects on Cancel.
   *
   */
  open(emails, owner) {
    this._emails = emails;
    this._owner = owner;
    this._render();
    this._dialog.showModal();
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
    this._dialog.close();
    this._reject();
  }

  _confirm() {
    this._dialog.close();
    this._resolve(this._selected);
  }

  _render() {
    render(template(this), this, { eventContext: this });
  }
});
