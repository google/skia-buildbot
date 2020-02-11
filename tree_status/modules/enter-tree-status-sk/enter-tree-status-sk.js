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

import { $$ } from 'common-sk/modules/dom'
import 'elements-sk/error-toast-sk'
import 'elements-sk/spinner-sk'


const template = (ele) => html`
<input id='tree_status' size=80 placeholder='Add tree status with text containing either of (open/close/caution)'></input>
<button @click=${ele._addTreeStatus}>Submit</button>
<br/>
<button @click=${ele._closeWithDep}>Caution with Dependency</button>
<button @click=${ele._closeWithDep}>Close with Dependency</button>
`;

define('enter-tree-status-sk', class extends ElementSk {
  constructor() {
    super(template);
  }

  connectedCallback() {
    super.connectedCallback();

    this._render();

    $$('#tree_status').addEventListener("keyup", e => {
      // Submit tree status when "Enter" is pressed.
      if (e.keyCode == 13) {
        e.preventDefault();
        this._addTreeStatus(e);
      }
    });
  }

  _closeWithDep(e) {
    console.log("Need to close with dependency. Open a popup here.");

    let treeStatus = $$('#tree_status', this);
    let detail = {message: treeStatus.value};
    this.dispatchEvent(new CustomEvent('close-tree-with-dep', { detail: detail, bubbles: true }));
    textarea.value = '';
  }

  _addTreeStatus(e) {
    let treeStatus = $$('#tree_status', this);
    let detail = {message: treeStatus.value};
    this.dispatchEvent(new CustomEvent('new-tree-status', { detail: detail, bubbles: true }));
    treeStatus.value = '';
  }

});
