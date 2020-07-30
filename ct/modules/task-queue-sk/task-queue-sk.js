/**
 * @fileoverview A custom element that loads the CT pending tasks queue and
 * displays it as a table.
 */

import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/icon/cancel-icon-sk';
import 'elements-sk/icon/check-circle-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/toast-sk';
import '../../../infra-sk/modules/confirm-dialog-sk';

import { $$, DomReady } from 'common-sk/modules/dom';
import { fromObject } from 'common-sk/modules/query';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { define } from 'elements-sk/define';
import { errorMessage } from 'elements-sk/errorMessage';
import { html } from 'lit-html';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import {
  getFormattedTimestamp, taskDescriptors, getTimestamp, getCtDbTimestamp,
} from '../ctfe_utils';

const template = (el) => html`
<table class="runssummary surface-themes-sk secondary-links" id=queue>
  <tr class=primary-variant-container-themes-sk>
    <th>Queue Position</th>
    <th>Added</th>
    <th>Task Type</th>
    <th>User</th>
    <th>Swarming Logs</th>
    <th>Request</th>
  </tr>
  ${el._pendingTasks.map((task, index) => taskRowTemplate(el, task, index))}
 </table>
${el._pendingTasks.map((task, index) => taskDetailDialogTemplate(task, index))}
<confirm-dialog-sk id=confirm_dialog></confirm-dialog-sk>
<toast-sk id=confirm_toast class=primary-variant-container-themes-sk duration=5000></toast-sk>
`;

const taskRowTemplate = (el, task, index) => html`
<tr>
  <td class=nowrap>
    ${index + 1}
    <delete-icon-sk title="Delete this task" alt=Delete ?hidden=${!task.canDelete}
      @click=${() => el.confirmDeleteTask(index)}></delete-icon-sk>
  </td>
  <td>
    ${getFormattedTimestamp(task.TsAdded)}
    ${task.FutureDate
    ? html`</br><span class=error-themes-sk>(scheduled in the future)</span>`
    : ''}
  </td>
  <td>${task.TaskType}</td>
  <td>${task.Username}</td>
  <td class=nowrap>${
  task.FutureDate
    ? html`N/A`
    : task.SwarmingLogs
      ? html`<a href="${task.SwarmingLogs}" rel=noopener target=_blank>Swarming Logs</a>`
      : html`No Swarming Logs`}</td>
  <td class=nowrap>
    <a href=# class=details
      @click=${() => el.showDetailsDialog(index)}>Task Details</a>
  </td>
</tr>`;

const taskDetailDialogTemplate = (task, index) => html`
<div id=${`detailsDialog${index}`} class="dialog-background hidden overlay-themes-sk"
  @click=${hideDialog}>
  <div class="dialog-content surface-themes-sk">
    <pre>${formatTask(task)}</pre>
  </div>
</div>
`;

function hideDialog(e) {
  if (e.target.classList.contains('dialog-background')) {
    e.target.classList.add('hidden');
  }
}

function formatTask(task) {
  return JSON.stringify(task, null, 4);
}

define('task-queue-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._pendingTasks = [];
    this._running = false;
  }

  connectedCallback() {
    super.connectedCallback();
    if (this._running) {
      return;
    }
    this._running = true;
    // We wait for everything to load so scaffolding event handlers are
    // attached.
    DomReady.then(() => {
      this.dispatchEvent(new CustomEvent('begin-task', { bubbles: true }));
      this._render();
      this.loadTaskQueue().then(() => {
        this._render();
        this._running = false;
        this.dispatchEvent(new CustomEvent('end-task', { bubbles: true }));
      });
    });
  }

  showDetailsDialog(index) {
    $$(`#detailsDialog${index}`, this).classList.remove('hidden');
  }

  // Dispatch requests to fetch tasks in queue. Returns a promise that resolves
  // when all task data fetching/updating is complete.
  loadTaskQueue() {
    this._pendingTasks = [];
    let queryParams = {
      size: 100,
      not_completed: true,
    };
    let queryStr = `?${fromObject(queryParams)}`;
    const allPromises = [];
    for (const obj of taskDescriptors) {
      allPromises.push(fetch(obj.get_url + queryStr, { method: 'POST' })
        .then(jsonOrThrow)
        .then((json) => {
          this.updatePendingTasks(json, obj);
        })
        .catch(errorMessage));
    }

    // Find all tasks scheduled in the future.
    queryParams = {
      include_future_runs: true,
    };
    queryStr = `?${fromObject(queryParams)}`;
    for (const obj of taskDescriptors) {
      allPromises.push(fetch(obj.get_url + queryStr, { method: 'POST' })
        .then(jsonOrThrow)
        .then((json) => {
          this.updatePendingTasks(json, obj);
        })
        .catch(errorMessage));
    }
    return Promise.all(allPromises);
  }

  // Add responses to pending tasks list.
  updatePendingTasks(json, taskDescriptor) {
    const tasks = json.data;
    for (let i = 0; i < tasks.length; i++) {
      const task = tasks[i];
      task.canDelete = json.permissions[i].DeleteAllowed;
      task.Id = json.ids[i];
      task.TaskType = taskDescriptor.type;
      task.GetURL = taskDescriptor.get_url;
      task.DeleteURL = taskDescriptor.delete_url;
      // Check if this is a completed task set to repeat.
      if (task.RepeatAfterDays !== 0 && task.TaskDone) {
        // Calculate the future date.
        const timestamp = getTimestamp(task.TsAdded);
        timestamp.setDate(timestamp.getDate() + task.RepeatAfterDays);
        task.FutureDate = true;
        task.TsAdded = getCtDbTimestamp(new Date(timestamp));
      }
    }
    this._pendingTasks = this._pendingTasks.concat(tasks);
    // Sort pending tasks according to TsAdded.
    this._pendingTasks.sort((a, b) => a.TsAdded - b.TsAdded);
  }


  confirmDeleteTask(index) {
    document.getElementById('confirm_dialog')
      .open('Proceed with deleting task?')
      .then(() => {
        this.deleteTask(index);
      })
      .catch(() => {});
  }

  deleteTask(index) {
    const pendingTask = this._pendingTasks[index];
    const params = {};
    params.id = pendingTask.Id;
    fetch(pendingTask.DeleteURL, { method: 'POST', body: JSON.stringify(params) })
      .then((res) => {
        if (res.ok) {
          this._pendingTasks.splice(index, 1);
          $$('#confirm_toast').innerText = `Deleted ${pendingTask.TaskType} task ${pendingTask.Id}`;
          $$('#confirm_toast').show();
          return;
        }
        // Non-OK status. Read the response and punt it to the catch.
        return res.text().then((text) => { throw `Failed to delete the task: ${text}`; });
      })
      .then(() => {
        this._render();
      })
      .catch(errorMessage);
  }
});
