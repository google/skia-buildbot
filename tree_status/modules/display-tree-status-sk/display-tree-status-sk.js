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

// States of the tree.
const OPEN = 'open';
const CAUTION = 'caution';
const CLOSED = 'closed';

function getLocalDate(timestamp) {
  return new Date(timestamp).toLocaleString();
}

function getStatusClass(message) {
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

function recentStatuses(ele) {
  return ele._statuses.map((status) => html`
<tr class=${getStatusClass(status.message)}>
  <td>${status.username}</td>
  <td>${getLocalDate(status.date)}</td>
  <td>${status.message}</td>
  <td>${status.rollers}</td>
</tr>`);
}

const template = (ele) => html`
<table class="recent_statuses">
  <tr>
    <th>Who</th>
    <th>When</th>
    <th>Message</th>
    <th>Wait for</th>
  </tr>
  ${recentStatuses(ele)}
</table>
`;

define('display-tree-status-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._statuses = [];
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  /** @prop statuses {Array<Object>} The list of recent tree statuses. */
  get statuses() { return this._statuses; }

  set statuses(val) {
    this._statuses = val;
    this._render();
  }
});
