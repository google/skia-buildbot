/**
 * @fileoverview The bulk of the Recreate PageSets Runs History and
 * Recreate Webpage Archives Runs History pages.
 */

import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/icon/redo-icon-sk';
import 'elements-sk/toast-sk';
import '../../../infra-sk/modules/confirm-dialog-sk';
import '../pagination-sk';

import { $$, DomReady } from 'common-sk/modules/dom';
import { fromObject } from 'common-sk/modules/query';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { define } from 'elements-sk/define';
import { errorMessage } from 'elements-sk/errorMessage';
import { html } from 'lit-html';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import {
  getFormattedTimestamp,
  formatRepeatAfterDays,
} from '../ctfe_utils';

const template = (el) => html`
<div>
  <h2>${el._contrainByUser ? 'My ' : ''}${el.taskType}</h2>
  <pagination-sk @page-changed=${(e) => el._pageChanged(e)}></pagination-sk>
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
    ${el._tasks.map((task, index) => taskRowTemplate(el, task, index))}
  </table>
</div>

<confirm-dialog-sk id=confirm_dialog></confirm-dialog-sk>
<toast-sk id=confirm_toast class=primary-variant-container-themes-sk duration=5000></toast-sk>
`;

const taskRowTemplate = (el, task, index) => html`
<tr>
  <!-- Id col -->
  <td class=nowrap>
    <span>${task.Id}</span>
    <delete-icon-sk title="Delete this task" alt=Delete ?hidden=${!task.canDelete}
      @click=${() => el._confirmDeleteTask(index)}></delete-icon-sk>
    <redo-icon-sk title="Redo this task" alt=Redo ?hidden=${!task.canRedo}
      @click=${() => el._confirmRedoTask(index)}></redo-icon-sk>
  </td>
  <!-- User col -->
  <td>${task.Username}</td>
  <!-- Timestamps col -->
  <td>
    <table class=inner-table>
      <tr>
        <td>Added:</td>
        <td class=nowrap>${getFormattedTimestamp(task.TsAdded)}</td>
      </tr>
      <tr>
        <td>Started:</td>
        <td class=nowrap>${getFormattedTimestamp(task.TsStarted)}</td>
      </tr>
      <tr>
        <td>Completed:</td>
        <td class=nowrap>${getFormattedTimestamp(task.TsCompleted)}</td>
      </tr>
    </table>
  </td>
  <!-- Task Config col -->
  <td>
    <table class=inner-table>
      <tr>
        <td>PageSet:</td>
        <td>${task.PageSets}</td>
      </tr>
    </table>
  </td>
  <!-- Results col -->
  <td class=nowrap>
    ${task.Failure ? html`<div class=error>Failed</div>` : ''}
    ${!task.TaskDone ? html`<div class=green>Waiting</div>` : ''}
    ${!task.Failure && task.TaskDone ? 'Done' : ''}
    ${task.SwarmingLogs ? html`
    <br/>
    <a href="${task.SwarmingLogs}" target=_blank rel="noopener noreferrer">
      Swarming Logs
    </a>` : ''}
  </td>
  <!-- Task Repeats -->
  <td>${formatRepeatAfterDays(task.RepeatAfterDays)}</td>
</tr>`;

define('admin-task-runs-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._upgradeProperty('taskType');
    this._upgradeProperty('getUrl');
    this._upgradeProperty('deleteUrl');
    this._upgradeProperty('redoUrl');
    this._tasks = [];
    this._constrainByUser = false;
    this._constrainByTest = true;
    this._resetPagination();
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
  get taskType() {
    return this.getAttribute('taskType');
  }

  set taskType(val) {
    this.setAttribute('taskType', val);
  }

  /**
   * @prop {string} getUrl - Specifies the URL to fetch tasks. Mirrors the
   * attribute.
   */
  get getUrl() {
    return this.getAttribute('getUrl');
  }

  set getUrl(val) {
    this.setAttribute('getUrl', val);
  }

  /**
   * @prop {string} deleteUrl - Specifies the URL to delete tasks. Mirrors the
   * attribute.
   */
  get deleteUrl() {
    return this.getAttribute('deleteUrl');
  }

  set deleteUrl(val) {
    this.setAttribute('deleteUrl', val);
  }

  /**
   * @prop {string} redoUrl - Specifies the URL to redo tasks. Mirrors the
   * attribute.
   */
  get redoUrl() {
    return this.getAttribute('redoUrl');
  }

  set redoUrl(val) {
    this.setAttribute('redoUrl', val);
  }

  _pageChanged(e) {
    this._pagination.offset = e.detail.offset;
    this._reload();
  }

  _reload() {
    this.dispatchEvent(new CustomEvent('begin-task', { bubbles: true }));
    this._tasks = [];
    const queryParams = {
      offset: this._pagination.offset,
      size: this._pagination.size,
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
      .then((json) => {
        this._tasks = json.data;
        this._pagination = json.pagination;
        $$('pagination-sk', this).pagination = this._pagination;
        for (let i = 0; i < this._tasks.length; i++) {
          this._tasks[i].canDelete = json.permissions[i].DeleteAllowed;
          this._tasks[i].canRedo = json.permissions[i].RedoAllowed;
          this._tasks[i].Id = json.ids[i];
        }
      })
      .catch(errorMessage)
      .finally(() => {
        this._render();
        this.dispatchEvent(new CustomEvent('end-task', { bubbles: true }));
      });
  }

  _confirmDeleteTask(index) {
    document.getElementById('confirm_dialog')
      .open('Proceed with deleting task?')
      .then(() => {
        this._deleteTask(index);
      })
      .catch(() => {});
  }

  _confirmRedoTask(index) {
    document.getElementById('confirm_dialog')
      .open('Reschedule this task?')
      .then(() => {
        this._redoTask(index);
      })
      .catch(() => {});
  }

  _deleteTask(index) {
    const params = {};
    params.id = this._tasks[index].Id;
    fetch(this.deleteUrl, { method: 'POST', body: JSON.stringify(params) })
      .then((res) => {
        if (res.ok) {
          $$('#confirm_toast').innerText = `Deleted task ${params.id}`;
          $$('#confirm_toast').show();
          return;
        }
        // Non-OK status. Read the response and punt it to the catch.
        res.text().then((text) => { throw `Failed to delete the task: ${text}`; });
      })
      .then(() => {
        this._reload();
      })
      .catch(errorMessage);
  }

  _redoTask(index) {
    const params = {};
    params.id = this._tasks[index].Id;
    fetch(this.redoUrl, { method: 'POST', body: JSON.stringify(params) })
      .then((res) => {
        if (res.ok) {
          $$('#confirm_toast').innerText = `Resubmitted task ${params.id}`;
          $$('#confirm_toast').show();
          return;
        }
        // Non-OK status. Read the response and punt it to the catch.
        res.text().then((text) => { throw `Failed to resubmit the task: ${text}`; });
      })
      .then(() => {
        this._reload();
      })
      .catch(errorMessage);
  }


  _resetPagination() {
    this._pagination = { offset: 0, size: 10 };
  }

  _constrainRunsByUser() {
    this._constrainByUser = !this._constrainByUser;
    this._resetPagination();
    this._reload();
  }

  _constrainRunsByTest() {
    this._constrainByTest = !this._constrainByTest;
    this._resetPagination();
    this._reload();
  }
});
