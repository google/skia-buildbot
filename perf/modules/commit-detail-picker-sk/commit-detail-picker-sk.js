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

import '../commit-detail-panel-sk'
import 'elements-sk/styles/buttons'

import { html, render } from 'lit-html'
import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import dialogPolyfill from 'dialog-polyfill'


function _titleFrom(ele) {
  const index = ele.selected;
  if (index === -1) {
    return 'Choose a commit.';
  }
  const d = ele._details[index];
  if (!d) {
    return 'Choose a commit.';
  }
  return `${d.author} -  ${d.message}`;
}

const template = (ele) => html`
  <button @click=${ele._open}>${_titleFrom(ele)}</button>
  <dialog>
    <commit-detail-panel-sk @commit-selected='${ele._panelSelect}' .details='${ele._details}' selectable selected=${ele.selected}></commit-detail-panel-sk>
    <button @click=${ele._close}>Close</button>
  </dialog>
`;

window.customElements.define('commit-detail-picker-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._details = [];
    this._title = 'Choose a commit.';
  }

  connectedCallback() {
    super.connectedCallback();
    this._upgradeProperty('details');
    this._render();
    this._dialog = this.querySelector('dialog');
    dialogPolyfill.registerDialog(this._dialog);
  }

  _panelSelect(e) {
    this._title = e.detail.description;
    this.selected = e.detail.selected;
    this._render();
  }

  _close() {
    this._dialog.close();
    this._render();
  }

  _open() {
    this._dialog.showModal();
    this._render();
  }

  static get observedAttributes() {
    return ['selected'];
  }

  /** @prop selected {string} Mirrors the selected attribute. */
  get selected() { return +this.getAttribute('selected'); }
  set selected(val) {
    this.setAttribute('selected', val);
  }

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

