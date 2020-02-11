/**
 * @module list-autorollers-sk
 * @description <h2><code>list-autorollers-sk</code></h2>
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
import 'elements-sk/checkbox-sk'

function autorollerRow(ele) {
  return ele._autorollers.map(autoroller => html`
<tr>
  <td>
    <checkbox-sk ?checked=${ele._checked.has(autoroller)} @click=${ele._clickHandler} id=${autoroller}></checkbox-sk>
  </td>
  <td>${autoroller}
  </td>
</tr>
`);
}

const template = (ele) => html`
<br/>
Will need a loop below after making a call to the backend.
<table class="autorollers">
  ${autorollerRow(ele)}
</table>
`;

function findParent(ele, tagName) {
  while (ele && (ele.tagName !== tagName)) {
    ele = ele.parentElement;
  }
  return ele;
}

define('list-autorollers-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._autorollers = ['abc', 'xyz'];
    this._checked = new Set();
  }

  _clickHandler(e) {
    let checkbox = findParent(e.target, 'CHECKBOX-SK');
    console.log("CLICKED ON");
    console.log(checkbox.id);
    console.log(checkbox);
    console.log(checkbox._input);
    console.log(checkbox._input.checked);
    if (checkbox._input.checked) {
      this._checked.add(checkbox.id)
    };
    // If it is checked then add it to the SET!
    console.log("so far these are logged");
    console.log(this._checked);
  }

  connectedCallback() {
    super.connectedCallback();

    this._render();
  }

});
