/**
 * @fileoverview The bulk of the Recreate PageSets Runs History and
 * Recreate Webpage Archives Runs History pages.
 */

import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/icon/redo-icon-sk';

import '../pagination-sk';

import { $$, DomReady } from 'common-sk/modules/dom';
import { fromObject } from 'common-sk/modules/query';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { define } from 'elements-sk/define';
import { errorMessage } from 'elements-sk/errorMessage';
import { html } from 'lit-html';
import { PaginationSk } from '../pagination-sk/pagination-sk';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import {
  getFormattedTimestamp,
  formatRepeatAfterDays,
} from '../ctfe_utils';
import {
  ResponsePagination,
  AdminDatastoreTask,
  RedoTaskRequest,
  DeleteTaskRequest,
  GetTasksResponse,
} from '../json';

export class AdminTaskRunsSk extends ElementSk {
  private _tasks: AdminDatastoreTask[] = [];

  private _constrainByUser = false;

  private _constrainByTest = true;

  private _running = false;

  private _pagination: ResponsePagination | null = null;

  constructor() {
    super(AdminTaskRunsSk.template);
    this._upgradeProperty('taskType');
    this._upgradeProperty('getUrl');
    this._upgradeProperty('deleteUrl');
    this._upgradeProperty('redoUrl');
    this._resetPagination();
  }

private static template = (el: AdminTaskRunsSk) => html`
<div>
  <h2>${el._constrainByUser ? 'My ' : ''}${el.taskType}</h2>
  <pagination-sk @page-changed=${(e: CustomEvent) => el._pageChanged(e)}></pagination-sk>
  <br/>
  <button id=userFilter @click=${() => el._constrainRunsByUser()}>
    ${el._constrainByUser ? 'View Everyone\'s Runs' : 'View Only My Runs'}
  </button>
  <button id=testFilter @click=${() => el._constrainRunsByTest()}>
    ${el._constrainByTest ? 'Include Test Run' : 'Exclude Test Runs'}
  </button>

  <br/>
  <br/>
  <table class="surface-themes-sk secondary-links runssummary" id=runssummary>
    <tr class=primary-variant-container-themes-sk>
      <th>Id</th>
      <th>User</th>
      <th>Timestamps</th>
      <th>Task Config</th>
      <th>Results</th>
      <th>Task Repeats</th>
    </tr>
    ${el._tasks.map((task: AdminDatastoreTask, index: number) => AdminTaskRunsSk.taskRowTemplate(el, task, index))}
  </table>
</div>
`;

private static taskRowTemplate = (el: AdminTaskRunsSk, task: AdminDatastoreTask, index: number) => html`
<tr>
  <!-- Id col -->
  <td class=nowrap>
    <span>${task.id}</span>
    <delete-icon-sk title="Delete this task" alt=Delete ?hidden=${!task.can_delete}
      @click=${() => el._confirmDeleteTask(index)}></delete-icon-sk>
    <redo-icon-sk title="Redo this task" alt=Redo ?hidden=${!task.can_redo}
      @click=${() => el._confirmRedoTask(index)}></redo-icon-sk>
  </td>
  <!-- User col -->
  <td>${task.username}</td>
  <!-- Timestamps col -->
  <td>
    <table class=inner-table>
      <tr>
        <td>Added:</td>
        <td class=nowrap>${getFormattedTimestamp(task.ts_added)}</td>
      </tr>
      <tr>
        <td>Started:</td>
        <td class=nowrap>${getFormattedTimestamp(task.ts_started)}</td>
      </tr>
      <tr>
        <td>Completed:</td>
        <td class=nowrap>${getFormattedTimestamp(task.ts_completed)}</td>
      </tr>
    </table>
  </td>
  <!-- Task Config col -->
  <td>
    <table class=inner-table>
      <tr>
        <td>PageSet:</td>
        <td>${task.page_sets}</td>
      </tr>
    </table>
  </td>
  <!-- Results col -->
  <td class=nowrap>
    ${task.failure ? html`<div class=error>Failed</div>` : ''}
    ${!task.task_done ? html`<div class=green>Waiting</div>` : ''}
    ${!task.failure && task.task_done ? 'Done' : ''}
    ${task.swarming_logs ? html`
    <br/>
    <a href="${task.swarming_logs}" target=_blank rel="noopener noreferrer">
      Swarming Logs
    </a>` : ''}
  </td>
  <!-- Task Repeats -->
  <td>${formatRepeatAfterDays(task.repeat_after_days)}</td>
</tr>`;

connectedCallback(): void {
  super.connectedCallback();
  if (this._running) {
    return;
  }
  this._running = true;
  // We wait for everything to load so scaffolding event handlers are
  // attached.
  DomReady.then(() => {
    this._render();
    this._reload().then(() => {
      this._running = false;
    });
  });
}

/**
   * @prop {string} taskType - Specifies the type of task. Mirrors the
   * attribute. Possible values include "RecreatePageSets" and
   * "RecreateWebpageArchives".
   */
get taskType(): string {
  return this.getAttribute('taskType') || '';
}

set taskType(val: string) {
  this.setAttribute('taskType', val);
}

/**
   * @prop {string} getUrl - Specifies the URL to fetch tasks. Mirrors the
   * attribute.
   */
get getUrl(): string {
  return this.getAttribute('getUrl') || '';
}

set getUrl(val: string) {
  this.setAttribute('getUrl', val);
}

/**
   * @prop {string} deleteUrl - Specifies the URL to delete tasks. Mirrors the
   * attribute.
   */
get deleteUrl(): string {
  return this.getAttribute('deleteUrl') || '';
}

set deleteUrl(val: string) {
  this.setAttribute('deleteUrl', val);
}

/**
   * @prop {string} redoUrl - Specifies the URL to redo tasks. Mirrors the
   * attribute.
   */
get redoUrl(): string {
  return this.getAttribute('redoUrl') || '';
}

set redoUrl(val: string) {
  this.setAttribute('redoUrl', val);
}

_pageChanged(e: CustomEvent): void {
  this._pagination!.offset = e.detail.offset;
  this._reload();
}

_reload(): Promise<void> {
  this.dispatchEvent(new CustomEvent('begin-task', { bubbles: true }));
  this._tasks = [];
  const queryParams = {
    offset: this._pagination!.offset,
    size: this._pagination!.size,
    filter_by_logged_in_user: false,
    exclude_dummy_page_sets: false,
  };
  if (this._constrainByUser) {
    queryParams.filter_by_logged_in_user = true;
  }
  if (this._constrainByTest) {
    queryParams.exclude_dummy_page_sets = true;
  }
  return fetch(`${this.getUrl}?${fromObject(queryParams)}`,
    { method: 'POST' })
    .then(jsonOrThrow)
    .then((json: GetTasksResponse) => {
      this._tasks = json.data;
      this._pagination = json.pagination;
      ($$('pagination-sk', this) as PaginationSk).pagination = this._pagination!;
      for (let i = 0; i < this._tasks.length; i++) {
        this._tasks[i].can_delete = json.permissions![i].DeleteAllowed;
        this._tasks[i].can_redo = json.permissions![i].RedoAllowed;
        this._tasks[i].id = json.ids![i];
      }
    })
    .catch(errorMessage)
    .finally(() => {
      this._render();
      this.dispatchEvent(new CustomEvent('end-task', { bubbles: true }));
    });
}

_confirmDeleteTask(index: number): void {
  const confirmed = window.confirm('Delete this task?');
  if (confirmed) {
    this._deleteTask(index);
  }
}

_confirmRedoTask(index: number): void {
  const confirmed = window.confirm('Reschedule this task?');
  if (confirmed) {
    this._redoTask(index);
  }
}

_deleteTask(index: number): void {
  const req: DeleteTaskRequest = { id: this._tasks[index].id };
  fetch(this.deleteUrl, { method: 'POST', body: JSON.stringify(req) })
    .then((res) => {
      if (res.ok) {
        window.alert(`Deleted task ${req.id}`);
        return;
      }
      // Non-OK status. Read the response and punt it to the catch.
      res.text().then((text) => { throw new Error(`Failed to delete the task: ${text}`); });
    })
    .then(() => {
      this._reload();
    })
    .catch(errorMessage);
}

_redoTask(index: number): void {
  const req: RedoTaskRequest = { id: this._tasks[index].id };
  fetch(this.redoUrl, { method: 'POST', body: JSON.stringify(req) })
    .then((res) => {
      if (res.ok) {
        window.alert(`Resubmitted task ${req.id}`);
        return;
      }
      // Non-OK status. Read the response and punt it to the catch.
      res.text().then((text) => { throw new Error(`Failed to resubmit the task: ${text}`); });
    })
    .then(() => {
      this._reload();
    })
    .catch(errorMessage);
}


_resetPagination(): void {
  this._pagination = { offset: 0, size: 10, total: 0 };
}

_constrainRunsByUser(): void {
  this._constrainByUser = !this._constrainByUser;
  this._resetPagination();
  this._reload();
}

_constrainRunsByTest(): void {
  this._constrainByTest = !this._constrainByTest;
  this._resetPagination();
  this._reload();
}
}

define('admin-task-runs-sk', AdminTaskRunsSk);
