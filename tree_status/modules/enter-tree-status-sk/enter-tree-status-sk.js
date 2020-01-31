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

const template = (ele) => html`
<div>hello world</div>
`;

define('enter-tree-status-sk', class extends ElementSk {
  constructor() {
    console.log("AAAA");
    super(template);
    console.log("BBBB");
    this._render();
  }

});
