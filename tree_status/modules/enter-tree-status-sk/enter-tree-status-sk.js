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
<textarea rows=2 cols=80 placeholder="Add tree status with text containing either of (open/close/caution)"></textarea>
<br/>
<button @click=${ele._addTreeStatus}>Submit</button>
<button @click=${ele._closeWithDep}>Close with Dependency</button>
`;

define('enter-tree-status-sk', class extends ElementSk {
  constructor() {
    super(template);
    Login.then(loginstatus => {
      this._user = loginstatus.Email;
      // this._render();
    });
  }

  // MOVE TO COMMON JS TO USE FROM DIFFERENT COMPONENTS>
  // Common work done for all fetch requests.
  _doImpl(url, detail, action=json => this._incidentAction(json)) {
    // this._busy.active = true;
    fetch(url, {
      body: JSON.stringify(detail),
      headers: {
        'content-type': 'application/json',
      },
      credentials: 'include',
      method: 'POST',
    }).then(jsonOrThrow).then(json => {
      action(json)
      this._render();
      // this._busy.active = false;
    }).catch(msg => {
      console.log("ERROR");
      console.log(msg);
      console.log(msg.resp);
      // this._busy.active = false;
      msg.resp.text().then(errorMessage);
    });
  }

  connectedCallback() {
    super.connectedCallback();

    this._render();
  }

  _closeWithDep(e) {
    console.log("Need to close with dependency. Open a popup here.");
  }

  _addTreeStatus(e) {
    console.log("IN _ADD TREE STASTUS")

    let textarea = $$('textarea', this);
    let detail = {message: textarea.value};
    this._doImpl('/_/add_tree_status', e.detail, _ => this.dispatchEvent(new CustomEvent('refresh-tree-status-display', {bubbles: true})));
    textarea.value = '';
  }

  /*
  _render() {
    render(template(this), this, {eventContext: this});
  }
  */

});
