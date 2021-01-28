/**
 * @module bot-chooser-sk
 * @description <h2><code>bot-chooser-sk</code></h2>
 *
 * <p>
 * This element pops up a dialog with OK and Cancel buttons. Its open method
 * returns a Promise which will resolve when the user clicks OK after selecting
 * a bot or reject when the user clicks Cancel.
 * </p>
 *
 */
import dialogPolyfill from 'dialog-polyfill';
import { define } from 'elements-sk/define';
import { html, render } from 'lit-html';
import { $$ } from 'common-sk/modules/dom';

import 'elements-sk/styles/buttons';
import 'elements-sk/styles/select';

function displayBotOptions(bots_to_incidents) {
  const botsHTML = [];
  for (const bot in bots_to_incidents) {
    botsHTML.push(html`
      <option value=${bot}>${bot} [${bots_to_incidents[bot].map((i) => i.params.alertname).join(',')}]</option>
    `);
  }
  return botsHTML;
}

function displayDialogContents(ele) {
  if (Object.keys(ele._bots_to_incidents).length === 0) {
    return html`
      <h2>No active bot alerts found</h2>
      <br/>
      <div class=buttons>
        <button @click=${ele._dismiss}>OK</button>
      </div>
    `;
  } else {
    return html`
      <h2>Bots with active alerts</h2>
      <select size=10 @input=${ele._input}>
        ${displayBotOptions(ele._bots_to_incidents)}
      </select>
      <div class=buttons>
        <button @click=${ele._dismiss}>Cancel</button>
        <button @click=${ele._confirm}>OK</button>
      </div>
    `
  }
}

const template = (ele) => html`<dialog>${displayDialogContents(ele)}</dialog>`;

define('bot-chooser-sk', class extends HTMLElement {
  constructor() {
    super();
    this._resolve = null;
    this._reject = null;
    this._bots_to_incidents = [];
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
   * @param bots_to_incidents {Object} Map of bots to their incidents.
   * @param bots_to_ignore {Array} Which bots should be ignored.
   * @returns {Promise} Returns a Promise that resolves on OK, and rejects on Cancel.
   *
   */
  open(bots_to_incidents, bots_to_ignore) {
    this._bots_to_incidents = {};
    for (const bot in bots_to_incidents) {
      if (bots_to_ignore.includes(bot)) {
        continue;
      }
      this._bots_to_incidents[bot] = bots_to_incidents[bot];
    }
    this._render();
    this._dialog.showModal();

    const selectElem = $$('select', this);
    if (selectElem) {
      selectElem.focus();
      if (selectElem.length > 0) {
        // Select the first option.
        this._selected = $$('select', this)[0].value;
        $$('select', this).selectedIndex = 0;
      }
    }
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
