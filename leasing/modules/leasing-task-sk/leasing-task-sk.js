/**
 * @module module/leasing-task-sk
 * @description <h2><code>leasing-task-sk</code></h2>
 *
 * <p>
 *   Displays information about a single leasing task.
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

import '../../../infra-sk/modules/login-sk';

function displayTaskStatus(task) {
  if (task.done) {
    return 'Completed';
  }
  return 'Still Running';
}

function formatTimestamp(timestamp) {
  if (!timestamp) {
    return timestamp;
  }
  const d = new Date(timestamp);
  return d.toUTCString();
}

function displayLeaseStartTime(task) {
  return displayLeaseTime(task.swarmingTaskState, task.leaseStartTime);
}

function displayLeaseEndTime(task) {
  return displayLeaseTime(task.swarmingTaskState, task.leaseEndTime);
}

function displayLeaseTime(taskState, taskTime) {
  if (taskState === 'PENDING') {
    return 'N/A';
  }
  return formatTimestamp(taskTime);
}

function displayDimensions(task) {
  let dims = task.osType;
  if (task.deviceType !== '') {
    dims += ` - ${device(task.deviceType)}${getAKAStr(task.deviceType)}`;
  }
  return html`
    <br/>
    Dimensions: ${dims}`;
}

function displayBotId(task) {
  if (!task.botId) {
    return '';
  }
  return html`
    <br/>
    Bot Id: <a href="https://${task.swarmingServer}/bot?id=${task.botId}" target="_blank">${task.botId}</a
  `;
}

function displaySwarmingTaskForIsolates(task) {
  if (!task.taskIdForIsolates) {
    return '';
  }
  return html`
    <br/>
    Task for Isolates: <a href="https://${task.swarmingServer}/task?id=${task.taskIdForIsolates}" target="_blank">Link</a
  `;
}

function displaySwarmingTask(task) {
  if (!task.swarmingTaskId) {
    return 'Processing';
  }
  return html`
    <a href="https://${task.swarmingServer}/task?id=${task.swarmingTaskId}" target="_blank">Link</a
  `;
}

function displayLeaseButtonRow(ele) {
  const task = ele._task;
  if (task.done || task.swarmingTaskState === 'PENDING') {
    return '';
  }
  return html`
  <tr>
    <td>
      <select id="duration">
        <option value="1" title="1 Hour">1hr</option>
        <option value="2" title="2 Hours">2hr</option>
        <option value="6" title="6 Hours">6hr</option>
        <option value="23" title="23 Hours">23hr</option>
      </select>
      <button raised @click=${ele._onExtend}>Extend Lease</button>
    </td>
    <td>
      <button raised @click=${ele._onExpire}>Expire Lease</button>
    </td>
  </tr>
`;
}

const template = (ele) => html`
  <table class="tasktable" cellpadding="5" border="1">
    <col width ="33%">
    <col width ="33%">
    <col width ="33%">

    <tr class="headers">
       <td colspan=2>
         Task - ${ele._task.description} - ${ele._task.requester} - ${displayTaskStatus(ele._task)}
       </td>
    </tr>

    <tr>
      <td>
        Created: ${formatTimestamp(ele._task.created)}
      </td>
      <td>
        Lease Start Time: ${displayLeaseStartTime(ele._task)}
        <br/>
        Lease End Time: ${displayLeaseEndTime(ele._task)}
      </td>
    </tr>

    <tr>
      <td>
        Pool: ${ele._task.pool}
        ${displayDimensions(ele._task)}
        ${displayBotId(ele._task)}
        ${displaySwarmingTaskForIsolates(ele._task)}
      </td>
      <td>
        Task Log:${displaySwarmingTask(ele._task)}
        <br/>
        Task Status: ${ele._task.swarmingTaskState}
      </td>
    </tr>

    ${displayLeaseButtonRow(ele)}
  </table>
`;

define('leasing-task-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._task = {};
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
