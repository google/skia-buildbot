/**
 * @module list-autorollers-sk
 * @description <h2><code>list-autorollers-sk</code></h2>
 *
 * <p>
 *   Displays a div that allows users to select a state (Caution/Closed) and
 *   a list of autorollers. Dynamically populates the tree status field with
 *   these selections. Also includes a submit button.
 * </p>
 *
 * @evt set-tree-status Sent when the user selects a state and list of
 *    autorollers. The detail includes the tree status message.
 *
 *    <pre>
 *      detail : "Caution: Waiting for Chrome, Flutter rollers to land"
 *    </pre>
 *
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { findParent } from 'common-sk/modules/dom';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import 'elements-sk/checkbox-sk';
import 'elements-sk/radio-sk';

function autorollerRow(ele) {
  return ele._autorollers.map((autoroller) => html`
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
  <radio-sk label="Caution" name=tree_state ?checked=${ele._selectedTreeStatus === 'Caution'} @change=${() => ele._radioHandler('Caution')}></radio-sk>
  <radio-sk label="Closed" name=tree_state ?checked=${ele._selectedTreeStatus === 'Closed'} @change=${() => ele._radioHandler('Closed')}></radio-sk>
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

define('list-autorollers-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._autorollers = [];
    this._checkedAutorollers = new Set();
    this._selectedTreeStatus = '';
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  /** @prop autorollers {string} The list of autorollers. */
  get autorollers() { return this._autorollers; }

  set autorollers(val) {
    this._autorollers = val;
    this._render();
  }

  // resets the textfield, radio buttons and check boxes.
  reset() {
    this._setTreeStatus('');
    this._checkedAutorollers = new Set();
    this._selectedTreeStatus = '';
    this.setAttribute('collapsed', '');
    this._render();
  }

  _submit() {
    this.dispatchEvent(new KeyboardEvent('keyup', { keyCode: 13 }));
  }

  getSelectedRollerNames() {
    return Array.from(this._checkedAutorollers).join(', ');
  }

  _clickHandler(e) {
    const checkbox = findParent(e.target, 'CHECKBOX-SK');
    if (checkbox._input.checked) {
      this._checkedAutorollers.add(checkbox.id);
    } else {
      this._checkedAutorollers.delete(checkbox.id);
    }
    this._setTreeStatus(this._getTreeStatusFromAutorollers());
  }

  _getTreeStatusFromAutorollers() {
    let treeStatus = this._selectedTreeStatus;
    const rollerNamesStr = this.getSelectedRollerNames();
    if (rollerNamesStr) {
      treeStatus = `${treeStatus}: Waiting for ${rollerNamesStr} roller${this._checkedAutorollers.size > 1 ? 's' : ''} to land`;
    }
    return treeStatus;
  }

  _setTreeStatus(treeStatus) {
    this.dispatchEvent(new CustomEvent('set-tree-status', { detail: treeStatus, bubbles: true }));
  }

  _radioHandler(state) {
    this._selectedTreeStatus = state;
    this._setTreeStatus(this._getTreeStatusFromAutorollers());
  }
});
