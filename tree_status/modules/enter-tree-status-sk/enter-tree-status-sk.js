/**
 * @module enter-tree-status-sk
 * @description <h2><code>enter-tree-status-sk</code></h2>
 *
 *   The main application element for am.skia.org.
 *
 */

import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import { define } from 'elements-sk/define'
import { html, render } from 'lit-html'
import { Login } from '../../../infra-sk/modules/login'
import { errorMessage } from 'elements-sk/errorMessage'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'

import '../list-autorollers-sk'

import { $$ } from 'common-sk/modules/dom'
import 'elements-sk/error-toast-sk'
import 'elements-sk/spinner-sk'


const template = (ele) => html`
<input id='tree_status' size=60 placeholder='Add tree status with text containing either of (open/close/caution)' value=${ele._status_value}></input>
<button @click=${ele._addTreeStatus}>Submit</button>
<br/>
<button id='display_autorollers' @click=${ele._displayAutorollers}>Caution/Close till Roll Lands</button>
<br/>
<br/>
<list-autorollers-sk .autorollers=${ele._autorollers} collapsable collapsed></list-autorollers-sk>
`;

define('enter-tree-status-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._autorollers = [];
    this._status_value = '';
  }

  connectedCallback() {
    super.connectedCallback();

    this._render();

    $$('#tree_status').addEventListener("keyup", e => {
      // Submit tree status when "Enter" is pressed in the text field.
      if (e.keyCode == 13) {
        e.preventDefault();
        this._addTreeStatus(e);
      }
    });

    $$('list-autorollers-sk').addEventListener("keyup", e => {
      // Submit tree status when "Enter" is pressed on the autorollers element.
      if (e.keyCode == 13) {
        e.preventDefault();
        this._addTreeStatus(e);
      }
    });
  }

  /** @prop autorollers {string} The list of autorollers. */
  get autorollers() { return this._autorollers }
  set autorollers(val) {
    this._autorollers = val;
    this._render();
  }

  /** @prop status_value {string} String to prefill the tree status text field with. */
  get status_value() { return this._status_value }
  set status_value(val) {
    $$('#tree_status', this).value = val;
    this._status_value = val;
    this._render();
  }

  _displayAutorollers(e) {
    const autorollersTable = $$('list-autorollers-sk')
    const treeStatusField = $$('#tree_status');
    const displayButton = $$('#display_autorollers');
    if (autorollersTable.hasAttribute('collapsed')) {
      autorollersTable.removeAttribute('collapsed');
      treeStatusField.setAttribute('disabled', '');
    } else {
      autorollersTable.reset();
      treeStatusField.removeAttribute('disabled');
    }
  }

  _addTreeStatus(e) {
    let treeStatus = $$('#tree_status', this);
    let detail = {message: treeStatus.value, rollers: $$('list-autorollers-sk').getSelectedRollerNames()};
    this.dispatchEvent(new CustomEvent('new-tree-status', { detail: detail, bubbles: true }));
    $$('list-autorollers-sk').reset();
    treeStatus.removeAttribute('disabled');
  }

});
