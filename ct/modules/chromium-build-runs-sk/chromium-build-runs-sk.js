/**
 * @fileoverview The bulk of the Chromium Builds Runs History page.
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
  shortHash,
  chromiumCommitUrl,
  skiaCommitUrl,
} from '../ctfe_utils';

const template = (el) => html`
<div>
  <h2>Chromium Builds Runs</h2>
  <pagination-sk @page-changed=${(e) => el._pageChanged(e)}></pagination-sk>
  <br/>
  <table class="surface-themes-sk secondary-links runssummary" id=runssummary>
    <tr class=primary-variant-container-themes-sk>
      <th>Id</th>
      <th>Chromium Commit Hash</th>
      <th>Submitted On</th>
      <th>Skia Commit Hash</th>
      <th>User</th>
      <th>Timestamps</th>
      <th>Results></th>
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
  <!-- Chromium Commit Hash col -->
  <td>
    <a href="${chromiumCommitUrl(task.ChromiumRev)}">
            ${shortHash(task.ChromiumRev)}
    </a>
  </td>
  <!-- Submitted On col -->
  <td class=nowrap>${getFormattedTimestamp(task.ChromiumRevTs)}</td>
  <!-- Skia Commit Hash col -->
  <td>
    <a href="${skiaCommitUrl(task.SkiaRev)}">
      ${shortHash(task.SkiaRev)}
    </a>
  </td>
  <!-- User col -->
  <td>${task.Username}</td>
  <!-- Timestamps col -->
  <td>
    <table class=inner-table>
      <tr>
        <td>Requested:</td>
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
  <!-- Results col -->
  <td class=nowrap>
    ${task.Failure ? html`<div class=error>Failed</div>` : ''}
    ${!task.TaskDone ? html`<div class=green>Waiting</div>` : ''}
    ${!task.Failure && task.TaskDone ? 'Done' : ''}
    ${task.Log ? html`
    <br/>
    <a href="${task.Log}" target=_blank rel="noopener noreferrer">
      log
    </a>`
    : ''}
    ${task.SwarmingLogs ? html`
    <br/>
    <a href="${task.SwarmingLogs}" target=_blank rel="noopener noreferrer">
      Swarming Logs
    </a>`
    : ''}
  </td>
  <!-- Task Repeats -->
  <td>${formatRepeatAfterDays(task.RepeatAfterDays)}</td>
</tr>`;

define('chromium-build-runs-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._tasks = [];
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
    return fetch(`/_/get_chromium_build_tasks?${fromObject(queryParams)}`,
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
    const note = index >= 0
      && this._tasks[index].TaskDone
      && !this._tasks[index].Failure
      ? ' Note: This build will no longer be available for running other tasks.' : '';
    document.getElementById('confirm_dialog')
      .open(`Proceed with deleting task?${note}`)
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
    fetch('/_/delete_chromium_build_task', { method: 'POST', body: JSON.stringify(params) })
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
    fetch('/_/redo_chromium_build_task', { method: 'POST', body: JSON.stringify(params) })
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
});
