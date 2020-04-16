/**
 * @module module/list-tasks-sk
 * @description <h2><code>list-tasks-sk</code></h2>
 *
 * <p>
 *   Displays information about all waiting and running tasks being processed
 *   by the Android Compile Server.
 * </p>
 *
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { $$ } from 'common-sk/modules/dom';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { upgradeProperty } from 'elements-sk/upgradeProperty';

import 'elements-sk/error-toast-sk';
import 'elements-sk/icon/folder-icon-sk';
import 'elements-sk/icon/gesture-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/icon/home-icon-sk';
import 'elements-sk/icon/star-icon-sk';
import 'elements-sk/nav-button-sk';
import 'elements-sk/nav-links-sk';

import { doImpl } from '../compile';

function formatTimestamp(timestamp) {
  if (!timestamp) {
    return timestamp;
  }
  const d = new Date(timestamp);
  return d.toLocaleString();
}

//inline this?
function getGerritLink(issue, patchset) {
  return `https://skia-review.googlesource.com/c/skia/+/$issue/$patchset`;
}

function getUnOwnedPendingTasksRows(ele) {
  return ele._unOwnedPendingTasks.map((task) => html`
  <tr>
    <td align="left">
      <a href="${getGerritLink(task.issue, task.patchset)}" target="_blank">skrev/${task.issue}/${task.patchset}</a> [${task.lunch_target}]
    </td>
    <td>
      Created: ${formatTimestamp(task.created)}
    </td>
  </tr>
  `);
}

function getOwnedPendingTasksRows(ele) {
  return ele._ownedPendingTasks.map((task) => html`
  <tr>
    <td align="left">
      Running on ${task.compile_server_instance} (${task.checkout}):
      <a href="${getGerritLink(task.issue, task.patchset)}" target="_blank">skrev/${task.issue}/${task.patchset}</a> [${task.lunch_target}]
    </td>
    <td>
      Created: ${formatTimestamp(task.created)}
    </td>
  </tr>
  `);
}

const template = (ele) => html`
  <table class="tasktable" cellpadding="5" border="1">
    <col width ="50%">
    <col width ="50%">

    <tr class="headers">
       <td colspan=2>
         ${ele._unOwnedPendingTasks.length} Tasks Waiting in Queue
       </td>
    </tr>
    ${getUnOwnedPendingTasksRows(ele)}
  </table>

  <br/><br/>
  <table class="tasktable" cellpadding="5" border="1">
    <col width ="50%">
    <col width ="50%">

    <tr class="headers">
       <td colspan=2>
         ${ele._ownedPendingTasks.length} Tasks Currently Running
       </td>
    </tr>
    ${getOwnedPendingTasksRows(ele)}
  </table>

`;

define('list-tasks-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._unOwnedPendingTasks = [];
    this._ownedPendingTasks = [];
    console.log("HER EHERE");
    this._fetchPendingTasks();
  }

  _fetchPendingTasks() {
    doImpl('/_/pending_tasks', {}, (json) => {
      this._unOwnedPendingTasks = json.unowned_pending_tasks;
      this._ownedPendingTasks = json.owned_pending_tasks;
      console.log(json);
      this._render();
    });
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  disconnectedCallback() {
    super.disconnectedCallback();
  }
});
