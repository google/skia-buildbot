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

import { AutorollerSnapshot } from '../json';

const template = (ele) => html`
<input id='tree_status' size=60 placeholder='Add tree status with text containing either of (open/close/caution)' value=${ele._status_value}></input>
<button @click=${ele._addTreeStatus}>Submit</button>
<br/>
<button id='display_autorollers' @click=${ele._toggleAutorollers}>Caution/Close till Roll Lands</button>
<br/>
<br/>
<list-autorollers-sk .autorollers=${ele._autorollers} collapsable collapsed></list-autorollers-sk>
`;

export class EnterTreeStatus extends ElementSk {
  // public autorollers: AutorollerSnapshot[] = [];

  // public status_value: string = '';

  constructor() {
    super(template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();

    $$('#tree_status')!.addEventListener('keyup', (e) => this.submitIfEnter(e as KeyboardEvent));
    $$('list-autorollers-sk')!.addEventListener('keyup', (e) => this.submitIfEnter(e as KeyboardEvent));
  }

  private submitIfEnter(e: KeyboardEvent): void {
    if (e.key === '13') {
      e.preventDefault();
      this.addTreeStatus();
    }
  }

  /** @prop autorollers {string} The list of autorollers. */
  get autorollers(): AutorollerSnapshot[] {
    return (this.getAttribute('autorollers') as unknown) as AutorollerSnapshot[];
  }

  set autorollers(val: AutorollerSnapshot[]) {
    this.setAttribute('autorollers', (val as unknown) as string);
  }

  /** @prop status_value {string} String to prefill the tree status text field with. */
  get status_value(): string {
    return this.getAttribute('status_value')!;
  }

  set status_value(val: string) {
    this.setAttribute('status_value', val);
  }

  static get observedAttributes(): string[] {
    return ['autorollers', 'status_value'];
  }

  // Toggles the autorollers element. The status field is cleared and enabled
  // when the element is collapsed. When the element is displayed the status
  // field is disabled.
  private toggleAutorollers() {
    const autorollersTable = $$('list-autorollers-sk');
    const treeStatusField = $$('#tree_status') as HTMLInputElement;
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
  private addTreeStatus() {
    const treeStatus = $$('#tree_status', this) as HTMLInputElement;
    const detail = { message: treeStatus.value, rollers: $$('list-autorollers-sk').getSelectedRollerNames() };
    this.dispatchEvent(new CustomEvent('new-tree-status', { detail: detail, bubbles: true }));
    $$('list-autorollers-sk').reset();
    treeStatus.removeAttribute('disabled');
  }
}

define('enter-tree-status-sk', EnterTreeStatus);
