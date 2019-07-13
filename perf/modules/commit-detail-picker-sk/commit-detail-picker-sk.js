/**
 * @module modules/commit-detail-picker-sk
 * @description <h2><code>commit-detail-picker-sk</code></h2>
 *
 * @evt commit-selected - Event produced when a commit is selected. The
 *     the event detail contains the serialized cid.CommitDetail.
 *
 *      {
 *        author: "foo (foo@example.org)",
 *        url: "skia.googlesource.com/bar",
 *        message: "Commit from foo.",
 *        ts: 1439649751,
 *      },
 *
 * @attr {Number} selected - The index of the selected commit.
 *
 */

import 'elements-sk/dialog-sk'
import '../commit-detail-panel-sk'
import 'elements-sk/styles/buttons'

import { html, render } from 'lit-html'
import { upgradeProperty } from 'elements-sk/upgradeProperty'
import { ElementSk } from '../../../infra-sk/modules/ElementSk'

const template = (ele) => html`
  <button @click=${ele._open}>${ele._title}</button>
  <dialog-sk>
    <commit-detail-panel-sk @commit-selected='${ele._panelSelect}' .details='${ele._details}' selectable selected=${ele.selected}></commit-detail-panel-sk>
    <button @click=${ele._close}>Close</button>
  </dialog-sk>
`;

window.customElements.define('commit-detail-picker-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._details = [];
    this._title = 'Choose a commit.';
  }

  connectedCallback() {
    super.connectedCallback();
    upgradeProperty(this, 'details');
    this._render();
    this._dialog = this.querySelector('dialog-sk');
  }

  _panelSelect(e) {
    this._title = e.detail.description;
    this._render();
  }

  _close() {
    this._dialog.shown = false;
    this._render();
  }

  _open() {
    this._dialog.shown = true;
    this._render();
  }

  static get observedAttributes() {
    return ['selected'];
  }

  /** @prop selected {string} Mirrors the selected attribute. */
  get selected() { return this.getAttribute('selected'); }
  set selected(val) { this.setAttribute('selected', val); }

  attributeChangedCallback(name, oldValue, newValue) {
    this._render();
  }

  /** @prop details {Array} An array of serialized cid.CommitDetail, e.g.
   *
   *  [
   *     {
   *       author: "foo (foo@example.org)",
   *       url: "skia.googlesource.com/bar",
   *       message: "Commit from foo.",
   *       ts: 1439649751,
   *     },
   *     ...
   *  ]
   */
  get details() { return this._details }
  set details(val) {
    this._details = val;
    this._render();
  }

});

