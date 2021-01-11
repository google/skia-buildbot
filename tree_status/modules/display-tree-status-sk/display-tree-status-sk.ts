/**
 * @module display-tree-status-sk
 * @description <h2><code>display-tree-status-sk</code></h2>
 *
 * <p>
 *   Displays recent tree statuses in a table with rows colored occording to
 *   the tree status (green=open,yellow=caution,red=closed).
 * </p>
 *
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { Status } from '../json';

// States of the tree.
const OPEN = 'open';
const CAUTION = 'caution';
const CLOSED = 'closed';

export class DisplayTreeStatusSk extends ElementSk {
  statusesData: Status[] = [];

  constructor() {
    super(DisplayTreeStatusSk.template);
  }

  private static template = (ele: DisplayTreeStatusSk) => html`
  <table class="recent_statuses">
    <tr>
      <th>Who</th>
      <th>When</th>
      <th>Message</th>
      <th>Wait for</th>
    </tr>
    ${ele.recentStatuses()}
  </table>
  `;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
  }

  private getLocalDate(timestamp: string) {
    return new Date(timestamp).toLocaleString();
  }

  private getStatusClass(message: string) {
    let treeState = '';
    const lowerCaseMessage = message.toLowerCase();
    if (lowerCaseMessage.includes(OPEN)) {
      treeState = OPEN;
    } else if (lowerCaseMessage.includes(CAUTION)) {
      treeState = CAUTION;
    } else if (lowerCaseMessage.includes(CLOSED)) {
      treeState = CLOSED;
    }
    return treeState;
  }

  private recentStatuses() {
    return this.statusesData.map((status) => html`
  <tr class=${this.getStatusClass(status.message)}>
    <td>${status.username}</td>
    <td>${this.getLocalDate(status.date)}</td>
    <td>${status.message}</td>
    <td>${status.rollers}</td>
  </tr>`);
  }

  /** @prop statuses {string} The list of recent tree statuses. */
  get statuses(): Status[] {
    return this.statusesData;
  }

  set statuses(val: Status[]) {
    this.statusesData = val;
    this._render();
  }
}

define('display-tree-status-sk', DisplayTreeStatusSk);
