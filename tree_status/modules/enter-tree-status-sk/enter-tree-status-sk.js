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

import 'elements-sk/error-toast-sk'
import 'elements-sk/spinner-sk'


const template = (ele) => html`
<div>hello world2</div>
`;

define('enter-tree-status-sk', class extends ElementSk {
  constructor() {
    super(template);
    console.log("AAA");
    this._user = '';
    Login.then(loginstatus => {
      this._user = loginstatus.Email;
      // this._render();
    });
  }

  connectedCallback() {                                                         
    super.connectedCallback();                                                  
    // upgradeProperty(this, 'cid');                                               
    this._render();                                                             
  } 

  /*
  _render() {
    render(template(this), this, {eventContext: this});
  }
  */

});
