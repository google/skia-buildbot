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

import 'elements-sk/error-toast-sk';
import 'elements-sk/icon/folder-icon-sk';
import 'elements-sk/icon/gesture-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/icon/home-icon-sk';
import 'elements-sk/icon/star-icon-sk';
import 'elements-sk/nav-button-sk';
import 'elements-sk/nav-links-sk';
import { device, getAKAStr, doImpl } from '../leasing';

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

function getUnownedPendingTasksRows(ele) {
  return ele._unownedPendingTasks.map((task) => html`
  <tr>
    <td align="left">
      <a href="${getGerritLink(task.issue, task.patchset)}" target="_blank">skrev/${task.issue}/${task.patchset}</a> ${task.lunch_target}
    </td>
    <td>
      Created: ${formatTimestamp(item.created)}
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
         ${ele._unownedPendingTasks.length} Tasks Waiting in Queue
       </td>
    </tr>
    ${getUnownedPendingTasksRows(ele)}
  </table>

  // ADD NEW TABLE  HERE.
  <br/><br/>


`;

define('compile-tasks-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._unownedPendingTasks = [];
    this._ownedPendingTasks = [];
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  _onExtend() {
    const confirmed = window.confirm('Proceed with extending leasing task?');
    if (!confirmed) {
      return;
    }
    const detail = {
      task: this._task.datastoreId,
      duration: parseInt($$('#duration', this).value, 10),
    };
    doImpl('/_/extend_leasing_task', detail, () => {
      window.location.href = '/my_leases';
    });
  }

  _onExpire() {
    const confirmed = window.confirm('Proceed with expiring leasing task?');
    if (!confirmed) {
      return;
    }
    const detail = {
      task: this._task.datastoreId,
    };
    doImpl('/_/expire_leasing_task', detail, () => {
      window.location.href = '/my_leases';
    });
  }

  /** @prop task {Object} Leasing task object. */
  get task() { return this._task; }

  set task(val) {
    this._task = val;
    this._render();
  }

  disconnectedCallback() {
    super.disconnectedCallback();
  }
});
