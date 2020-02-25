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

import { $, $$ } from 'common-sk/modules/dom'
import 'elements-sk/error-toast-sk'
import 'elements-sk/spinner-sk'
import 'elements-sk/checkbox-sk'
import 'elements-sk/radio-sk'

function autorollerRow(ele) {
  return ele._autorollers.map(autoroller => html`
<tr>
  <td>
    <checkbox-sk ?checked=${ele._checkedAutorollers.has(autoroller.name)} @click=${ele._clickHandler} id=${autoroller.name}></checkbox-sk>
  </td>
  <td>
    <a href="${autoroller.url}" target="_blank">${autoroller.name}</a>
  </td>
  <td>
    [Failing: ${autoroller.num_failed}]
  </td>
</tr>
`);
}

const template = (ele) => html`
<div class="autorollers">
  Choose state:
  <hr/>
  <radio-sk label="Caution" name=tree_state ?checked=${ele._selectedTreeStatus === "Caution"} @change=${() => ele._radioHandler("Caution")}></radio-sk>
  <radio-sk label="Closed" name=tree_state ?checked=${ele._selectedTreeStatus === "Closed"} @change=${() => ele._radioHandler("Closed")}></radio-sk>
  <br/><br/>
  Choose rollers to wait for:
  <hr/>
  <table>
    ${autorollerRow(ele)}
    <tr>
      <td colspan=3 class="submit-button">
        <br/>
        <button @click=${ele._submit}>Submit</button>
      </td>
    </tr>
  </table>
</div>
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
    this._autorollers = [];
    this._checkedAutorollers = new Set();
    this._selectedTreeStatus = '';
  }

  /** @prop autorollers {string} The list of autorollers. */
  get autorollers() { return this._autorollers }
  set autorollers(val) {
    this._autorollers = val;
    this._render();
  }

  // resets the textfield, radio buttons and check boxes.
  reset() {
    this.setTreeStatus('');
    this._checkedAutorollers = new Set();
    this._selectedTreeStatus = '';
    this.setAttribute('collapsed', '');
    this._render();
  }

  _submit() {
    this.dispatchEvent(new KeyboardEvent('keyup', {'keyCode': 13}))
  }

  getSelectedRollerNames() {
    return Array.from(this._checkedAutorollers).join(', ');
  }

  _clickHandler(e) {
    let checkbox = findParent(e.target, 'CHECKBOX-SK');
    if (checkbox._input.checked) {
      this._checkedAutorollers.add(checkbox.id);
    } else {
      this._checkedAutorollers.delete(checkbox.id);
    }
    this.setTreeStatus(this._getTreeStatusFromAutorollers())
  }

  _getTreeStatusFromAutorollers() {
    let treeStatus = this._selectedTreeStatus;
    const rollerNamesStr = this.getSelectedRollerNames();;
    if (rollerNamesStr) {
      treeStatus = `${treeStatus}: Waiting for ${rollerNamesStr} roller${this._checkedAutorollers.size> 1 ? 's' : ''} to land`;
    }
    return treeStatus;
  }

  setTreeStatus(treeStatus) {
    this.dispatchEvent(new CustomEvent('set-tree-status', { detail: treeStatus, bubbles: true }));
  }

  _radioHandler(state) {
    this._selectedTreeStatus = state;
    this.setTreeStatus(this._getTreeStatusFromAutorollers())
  }

  connectedCallback() {
    super.connectedCallback();

    this._render();
  }

});
