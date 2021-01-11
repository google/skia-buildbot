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
import { CheckOrRadio } from 'elements-sk/checkbox-sk/checkbox-sk';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { AutorollerSnapshot } from '../json';

import 'elements-sk/checkbox-sk';
import 'elements-sk/radio-sk';

export class ListAutorollersSk extends ElementSk {
  private checkedAutorollers: Set<string> = new Set();

  private autorollersData: AutorollerSnapshot[] = [];

  private selectedTreeStatus: string = '';

  constructor() {
    super(ListAutorollersSk.template);
  }

  private static template = (ele: ListAutorollersSk) => html`
  <div class="autorollers">
    Choose state:
    <hr/>
    <radio-sk label="Caution" name=tree_state ?checked=${ele.selectedTreeStatus === 'Caution'} @change=${() => ele.radioHandler('Caution')}></radio-sk>
    <radio-sk label="Closed" name=tree_state ?checked=${ele.selectedTreeStatus === 'Closed'} @change=${() => ele.radioHandler('Closed')}></radio-sk>
    <br/><br/>
    Choose rollers to wait for:
    <hr/>
    <table>
      ${ele.autorollerRow()}
      <tr>
        <td colspan=3 class="submit-button">
          <br/>
          <button @click=${ele.submit}>Submit</button>
        </td>
      </tr>
    </table>
  </div>
  `;

  // resets the textfield, radio buttons and check boxes.
  public reset(): void {
    this.setTreeStatus('');
    this.checkedAutorollers = new Set();
    this.selectedTreeStatus = '';
    this.setAttribute('collapsed', '');
    this._render();
  }

  public getSelectedRollerNames(): string {
    return Array.from(this.checkedAutorollers).join(', ');
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  private autorollerRow() {
    return this.autorollers.map((autoroller) => html`
  <tr>
    <td>
      <checkbox-sk ?checked=${this.checkedAutorollers.has(autoroller.name)} @change=${this.clickHandler} id=${autoroller.name}></checkbox-sk>
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

  // The list of autorollers.
  get autorollers(): AutorollerSnapshot[] {
    return this.autorollersData;
  }

  set autorollers(val: AutorollerSnapshot[]) {
    this.autorollersData = val;
    this._render();
  }

  private submit() {
    this.dispatchEvent(new KeyboardEvent('keyup', { key: 'Enter' }));
  }

  private clickHandler(e: Event) {
    const checkbox = findParent(e.target as HTMLElement, 'CHECKBOX-SK') as CheckOrRadio;
    if (checkbox.checked) {
      this.checkedAutorollers.add(checkbox.id);
    } else {
      this.checkedAutorollers.delete(checkbox.id);
    }
    this.setTreeStatus(this.getTreeStatusFromAutorollers());
  }

  private getTreeStatusFromAutorollers(): string {
    let treeStatus = this.selectedTreeStatus;
    const rollerNamesStr = this.getSelectedRollerNames();
    if (rollerNamesStr) {
      treeStatus = `${treeStatus}: Waiting for ${rollerNamesStr} roller${this.checkedAutorollers.size > 1 ? 's' : ''} to land`;
    }
    return treeStatus;
  }

  private setTreeStatus(treeStatus: string) {
    this.dispatchEvent(new CustomEvent('set-tree-status', { detail: treeStatus, bubbles: true }));
  }

  private radioHandler(state: string) {
    this.selectedTreeStatus = state;
    this.setTreeStatus(this.getTreeStatusFromAutorollers());
  }
}

define('list-autorollers-sk', ListAutorollersSk);
