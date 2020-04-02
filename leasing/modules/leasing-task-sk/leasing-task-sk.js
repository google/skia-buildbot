/**
 * @module module/leasing-task-sk
 * @description <h2><code>leasing-task-sk</code></h2>
 *
 * <p>
 *   Contains the title bar and error-toast for all the leasing server pages.
 *   The rest of pages should be a child of this element.
 * </p>
 *
 */

import { define } from 'elements-sk/define';
import { html } from 'lit-html';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import 'elements-sk/error-toast-sk';
import 'elements-sk/icon/folder-icon-sk';
import 'elements-sk/icon/gesture-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/icon/home-icon-sk';
import 'elements-sk/icon/star-icon-sk';
import 'elements-sk/nav-button-sk';
import 'elements-sk/nav-links-sk';
import { device, getAKAStr, doImpl } from '../leasing'

import '../../../infra-sk/modules/login-sk';

function displayTaskStatus(task) {
  if (task.done) {
    return 'Completed';
  } else {
    return 'Still Running';
  }
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
  } else {
    return formatTimestamp(taskTime);
  }
}

function displayDimensions(task) {
   let dims = task.osType;                                                    
   if (task.deviceType != '') {                                              
     dims += ' - ' + device(task.deviceType) + getAKAStr(task.deviceType);
      }
  return html`<br/>${dims}`
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

function displayLeaseButtons(task) {
  return html`
<tr><td>TODO TODO TODO</td></tr>
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

    ${displayLeaseButtons(ele._task)}
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

  /** @prop task {Object} Leasing task object. */
  get task() { return this._task; }

  set task(val) {
    console.log("THIS IS SET");
    console.log(val);
    this._task = val;
    this._render();
  }

  disconnectedCallback() {
    super.disconnectedCallback();
  }
});
