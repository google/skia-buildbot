import { upgradeProperty } from 'skia-elements/dom'
import { html, render } from 'lit-html/lib/lit-extended'

import 'skia-elements/dialog-sk'
import 'skia-elements/buttons'

const template = (ele) => html`<dialog-sk>
  <h2>Confirm</h2>
  <div class=message>${ele._message}</div>
  <div class=buttons>
  <button on-click=${e => ele._dismiss()}>Cancel</button>
  <button on-click=${e => ele._confirm()}>OK</button>
  </div>
</dialog-sk>`;

// The <confirm-dialog-sk> custom element declaration.
//
// This element pops up a dialog with OK and Cancel buttons. Its open method returns a Promise
// which will resolve when the user clicks OK or reject when the user clicks Cancel.
//
// Example:
//
//    <confirm-dialog-sk id="confirm_dialog"></confirm-dialog-sk>
//
//    <script>
//      (function(){
//        $('confirm-dialog').open("Proceed with taking over the world?").then(() => {
//          // Do some thing on confirm.
//        }).catch(() => {
//          // Do some thing on cancel.
//        });
//      })();
//    </script>
//
//
//  Attributes:
//    None
//
//  Properties:
//    None
//
//  Events:
//    None
//
//  Methods:
//    open(message) - Returns a Promise that resolves on OK, and rejects on Cancel.
//       message (string) - Message to display. Text only, any markup will be escaped.
//
window.customElements.define('confirm-dialog-sk', class extends HTMLElement {
  constructor() {
    super();
    this._promise = null;
    this._resolve = null;
    this._reject = null;
    this._message = '';
  }

  connectedCallback() {
    this._render();
  }

  open(message) {
    this._message = message;
    this._render();
    this.firstChild.show();
    this._promise = new Promise((resolve, reject) => {
      this._resolve = resolve;
      this._reject = reject;
    });
    return this._promise;
  }

  _dismiss() {
    this.firstChild.hide();
    this._reject();
  }

  _confirm() {
    this.firstChild.hide();
    this._resolve();
  }

  _render() {
    render(template(this), this);
  }

});

