/**
 * @module enter-tree-status-sk
 * @description <h2><code>enter-tree-status-sk</code></h2>
 *
 * <p>
 *   Displays a text field to enter the desired tree status into.
 *   Also contains the list-autorollers-sk element to update status with
 *   list of autorollers to wait for.
 * </p>
 *
 * @evt new-tree-status Sent when the user is done entering a new tree status.
 *    The detail includes the tree status message and the list of selected
 *    autorollers.
 *
 *    <pre>
 *      detail {
 *        message: "Tree is open",
 *        rollers: "Chrome, Flutter",
 *      }
 *    </pre>
 *
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';

import '../list-autorollers-sk';

import { $$ } from 'common-sk/modules/dom';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

const template = (ele) => html`
<input id='tree_status' size=60 placeholder='Add tree status with text containing either of (open/close/caution)' value=${ele._status_value}></input>
<button @click=${ele._addTreeStatus}>Submit</button>
<br/>
<button id='display_autorollers' @click=${ele._toggleAutorollers}>Caution/Close till Roll Lands</button>
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

    $$('#tree_status').addEventListener('keyup', (e) => this.submitIfEnter(e));
    $$('list-autorollers-sk').addEventListener('keyup', (e) => this.submitIfEnter(e));
  }

  submitIfEnter(e) {
    if (e.keyCode === 13) {
      e.preventDefault();
      this._addTreeStatus(e);
    }
  }

  /** @prop autorollers {string} The list of autorollers. */
  get autorollers() { return this._autorollers; }

  set autorollers(val) {
    this._autorollers = val;
    this._render();
  }

  /** @prop status_value {string} String to prefill the tree status text field with. */
  get status_value() { return this._status_value; }

  set status_value(val) {
    $$('#tree_status', this).value = val;
    this._status_value = val;
    this._render();
  }

  // Toggles the autorollers element. The status field is cleared and enabled
  // when the element is collapsed. When the element is displayed the status
  // field is disabled.
  _toggleAutorollers() {
    const autorollersTable = $$('list-autorollers-sk');
    const treeStatusField = $$('#tree_status');
    if (autorollersTable.hasAttribute('collapsed')) {
      autorollersTable.removeAttribute('collapsed');
      treeStatusField.setAttribute('disabled', '');
    } else {
      autorollersTable.reset();
      treeStatusField.removeAttribute('disabled');
    }
  }

  // Sends the new-tree-status event with the tree status message and list of
  // autorollers when called.
  _addTreeStatus() {
    const treeStatus = $$('#tree_status', this);
    const detail = { message: treeStatus.value, rollers: $$('list-autorollers-sk').getSelectedRollerNames() };
    this.dispatchEvent(new CustomEvent('new-tree-status', { detail: detail, bubbles: true }));
    $$('list-autorollers-sk').reset();
    treeStatus.removeAttribute('disabled');
  }
});
