/**
 * @fileoverview A custom element that loads the CT pending tasks queue and
 * displays it as a table.
 */

import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/icon/cancel-icon-sk';
import 'elements-sk/icon/check-circle-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/toast-sk';

import { $$, DomReady } from 'common-sk/modules/dom';
import { fromObject } from 'common-sk/modules/query';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { define } from 'elements-sk/define';
import { errorMessage } from 'elements-sk/errorMessage';
import { html } from 'lit-html';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import {
  getFormattedTimestamp, taskDescriptors, getTimestamp, getCtDbTimestamp, TaskDescriptor,
} from '../ctfe_utils';

import {
  CommonCols,
  DeleteTaskRequest,
  GetTasksResponse,
} from '../json';

function hideDialog(e: Event) {
  const classList = (e.target as HTMLElement).classList;
  if (classList.contains('dialog-background')) {
    classList.add('hidden');
  }
}

function formatTask(task: CommonCols) {
  return JSON.stringify(task, null, 4);
}

export class TaskQueueSk extends ElementSk {
  private _pendingTasks: CommonCols[] = [];

  private _running: boolean = false;

  constructor() {
    super(TaskQueueSk.template);
  }

  private static template = (el: TaskQueueSk) => html`
  <table class="runssummary surface-themes-sk secondary-links" id=queue>
    <tr class=primary-variant-container-themes-sk>
      <th>Queue Position</th>
      <th>Added</th>
      <th>Task Type</th>
      <th>User</th>
      <th>Swarming Logs</th>
      <th>Request</th>
    </tr>
    ${el._pendingTasks.map((task: CommonCols, index: number) => TaskQueueSk.taskRowTemplate(el, task, index))}
   </table>
  ${el._pendingTasks.map((task, index) => TaskQueueSk.taskDetailDialogTemplate(task, index))}
  <toast-sk id=confirm_toast class=primary-variant-container-themes-sk duration=5000></toast-sk>
  `;

  private static taskRowTemplate = (el: TaskQueueSk, task: CommonCols, index: number) => html`
  <tr>
    <td class=nowrap>
      ${index + 1}
      <delete-icon-sk title="Delete this task" alt=Delete ?hidden=${!task.can_delete}
        @click=${() => el.confirmDeleteTask(index)}></delete-icon-sk>
    </td>
    <td>
      ${getFormattedTimestamp(task.ts_added)}
      ${task.future_date
    ? html`</br><span class=error-themes-sk>(scheduled in the future)</span>`
    : ''}
    </td>
    <td>${task.task_type}</td>
    <td>${task.username}</td>
    <td class=nowrap>${
    task.future_date
      ? html`N/A`
      : task.swarming_logs
        ? html`<a href="${task.swarming_logs}" rel=noopener target=_blank>Swarming Logs</a>`
        : html`No Swarming Logs`}</td>
    <td class=nowrap>
      <a href=# class=details
        @click=${() => el.showDetailsDialog(index)}>Task Details</a>
    </td>
  </tr>`;

  private static taskDetailDialogTemplate = (task: CommonCols, index: number) => html`
  <div id=${`detailsDialog${index}`} class="dialog-background hidden overlay-themes-sk"
    @click=${hideDialog}>
    <div class="dialog-content surface-themes-sk">
      <pre>${formatTask(task)}</pre>
    </div>
  </div>
  `;

  connectedCallback(): void {
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

  showDetailsDialog(index: number): void {
    ($$(`#detailsDialog${index}`, this) as HTMLElement).classList.remove('hidden');
  }

  // Dispatch requests to fetch tasks in queue. Returns a promise that resolves
  // when all task data fetching/updating is complete.
  loadTaskQueue(): Promise<void[]> {
    this._pendingTasks = [];
    // TODO(rmistry):
    // This should really use the task_common.QueryParams type and
    // the parameters should be POST'ed instead of sent via queryStr.
    const queryParams = {
      size: 100,
      not_completed: true,
      include_future_runs: false,
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
    const futureQueryParams = {
      include_future_runs: true,
    };
    queryStr = `?${fromObject(futureQueryParams)}`;
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
  updatePendingTasks(json: GetTasksResponse, taskDescriptor: TaskDescriptor): void {
    const tasks = json.data;
    for (let i = 0; i < tasks.length; i++) {
      const task = tasks[i] as CommonCols;
      task.can_delete = json.permissions![i].DeleteAllowed;
      task.id = json.ids![i];
      task.task_type = taskDescriptor.type;
      task.get_url = taskDescriptor.get_url;
      task.delete_url = taskDescriptor.delete_url;
      // Check if this is a completed task set to repeat.
      if (task.repeat_after_days !== 0 && task.task_done) {
        // Calculate the future date.
        const timestamp = getTimestamp(task.ts_added);
        timestamp.setDate(timestamp.getDate() + task.repeat_after_days);
        task.future_date = true;
        task.ts_added = +getCtDbTimestamp(new Date(timestamp));
      }
    }
    this._pendingTasks = this._pendingTasks.concat(tasks);
    // Sort pending tasks according to TsAdded.
    this._pendingTasks.sort((a, b) => a.ts_added - b.ts_added);
  }

  confirmDeleteTask(index: number): void {
    const confirmed = window.confirm('Delete this task?');
    if (confirmed) {
      this.deleteTask(index);
    }
  }

  deleteTask(index: number): void {
    const pendingTask = this._pendingTasks[index];
    const params: DeleteTaskRequest = { id: pendingTask.id };
    // params.id = pendingTask.Id;
    fetch(pendingTask.delete_url, { method: 'POST', body: JSON.stringify(params) })
      .then((res) => {
        if (res.ok) {
          this._pendingTasks.splice(index, 1);
          window.alert(`Deleted ${pendingTask.task_type} task ${pendingTask.id}`);
          return;
        }
        // Non-OK status. Read the response and punt it to the catch.
        res.text().then((text) => { throw new Error(`Failed to delete the task: ${text}`); });
      })
      .then(() => {
        this._render();
      })
      .catch(errorMessage);
  }
}

define('task-queue-sk', TaskQueueSk);
