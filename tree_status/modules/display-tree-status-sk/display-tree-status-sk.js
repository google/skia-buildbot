/**
 * @module display-tree-status-sk
 * @description <h2><code>display-tree-status-sk</code></h2>
 *
 *   Displays the recent tree statuses.
 *
 */

import { ElementSk } from '../../../infra-sk/modules/ElementSk'
import { define } from 'elements-sk/define'
import { html, render } from 'lit-html'
import { errorMessage } from 'elements-sk/errorMessage'
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow'

import 'elements-sk/error-toast-sk'

// States of the tree.
const OPEN = 'open';
const CAUTION = 'caution';
const CLOSED = 'closed';


function recentStatuses(ele) {
  return ele._statuses.map(status => html`<tr class=${getStatusClass(status.message)}><td>${status.username}</td><td>${getLocalDate(status.date)}</td><td>${status.message}</td></tr>`);
}

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

const template = (ele) => html`
<table class="recent_statuses">
  <tr>
    <th>Who</th>
    <th>When</th>
    <th>Message</th>
  </tr>
  ${recentStatuses(ele)}
</table>
`;

define('display-tree-status-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._statuses = [];
  }

  /** @prop statuses {string} The list of recent tree statuses. */
  get statuses() { return this._statuses }
  set statuses(val) {
    this._statuses = val;
    this._render();
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

});
