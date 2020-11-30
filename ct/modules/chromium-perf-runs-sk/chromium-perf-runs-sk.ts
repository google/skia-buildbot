/**
 * @fileoverview The bulk of the Chromium Perf Runs History page.
 */

import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/icon/redo-icon-sk';
import 'elements-sk/icon/cancel-icon-sk';
import 'elements-sk/icon/check-circle-icon-sk';
import 'elements-sk/icon/help-icon-sk';
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
  getFormattedTimestamp, getGSLink, isEmptyPatch, formatRepeatAfterDays,
} from '../ctfe_utils';
import {
  ResponsePagination,
  ChromiumPerfDatastoreTask,
  RedoTaskRequest,
  DeleteTaskRequest,
} from '../json';

const hideDialog = (e: Event) => {
  if (e.target.classList.contains('dialog-background')) {
    e.target.classList.add('hidden');
  }
};

export class ChromiumPerfRunsSk extends ElementSk {
  private _tasks: ChromiumPerfDatastoreTask[] = [];

  private _constrainByUser = false;

  private _constrainByTest = true;

  private _running = false;

  private _pagination: ResponsePagination | null = null;

  constructor() {
    super(ChromiumPerfRunsSk.template);
    this._resetPagination();
  }

  private static template = (el: ChromiumPerfRunsSk) => html`
  <div>
    <h2>${el._contraintByUser ? 'My ' : ''}Chromium Perf Runs</h2>
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
        <th>Description</th>
        <th>Results</th>
        <th>Arguments</th>
        <th>Patches</th>
        <th>Task Repeats</th>
      </tr>
      ${el._tasks.map((task, index) => ChromiumPerfRunsSk.taskRowTemplate(el, task, index))}
    </table>
  </div>

  ${el._tasks.map((task, index) => ChromiumPerfRunsSk.taskDialogTemplate(task, index))}
  <confirm-dialog-sk id=confirm_dialog></confirm-dialog-sk>
  <toast-sk id=confirm_toast class=primary-variant-container-themes-sk duration=5000></toast-sk>
  `;

  private static taskRowTemplate = (el: ChromiumPerfRunsSk, task: ChromiumPerfDatastoreTask, index: number) => html`
  <tr>
    <!-- Id col -->
    <td class=nowrap>
      ${task.Results
    ? html`<a href="${task.results}" target=_blank rel="noopener noreferrer">${task.id}</a>`
    : html`<span>${task.Id}</span>`}
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
          <td>Benchmark:</td>
          <td>${task.benchmark}</td>
        </tr>
        <tr>
          <td>Platform:</td>
          <td>${task.platform}</td>
        </tr>
        <tr>
          <td>PageSet:</td>
          <td>
            ${!isEmptyPatch(task.custom_webpages_gspath)
      ? html`<a href="${getGSLink(task.custom_webpages_gspath)}"
                target=_blank rel="noopener noreferrer">Custom Webpages</a>`
      : task.page_sets}
          </td>
        </tr>
        <tr>
          <td>Repeats:</td>
          <td>${task.repeat_runs}</td>
        </tr>
        <tr>
          <td>ParallelRun:</td>
          <td>${task.run_in_parallel}</td>
        </tr>
        ${task.value_column_name
        ? html`<tr>
            <td class=nowrap>Value Column:</td>
            <td class=nowrap>${task.value_column_name}</td>
            </tr>`
        : ''}
        ${task.task_priority
          ? html`<tr>
            <td>TaskPriority:</td>
            <td>${task.task_priority}</td>
          </tr>`
          : ''}
        ${task.CCList
            ? html`<tr>
            <td>CC List:</td>
            <td>${task.CCList}</td>
          </tr>`
            : ''}
        ${task.GroupName
              ? html`<tr>
            <td>GroupName:</td>
            <td><a href="https://ct-perf.skia.org/e/?request_type=1">${task.GroupName}</a></td>
          </tr>`
              : ''}
        ${task.ChromiumHash
                ? html`<tr>
            <td>ChromiumHash:</td>
            <td><a href="https://chromium.googlesource.com/chromium/src/+show/${task.ChromiumHash}">${task.ChromiumHash}</a></td>
          </tr>`
                : ''}
      </table>
    </td>

    <!-- Description col -->
    <td>${task.description}</td>

    <!-- Results col -->
    <td class=nowrap>
      ${task.failure ? html`<div class=error>Failed</div>` : ''}
      ${!task.task_done ? html`<div class=green>Waiting</div>` : ''}
      ${task.results
                  ? html`<a href="${task.results}" target=_blank rel="noopener noreferrer">
          Overall Result
        </a>
        <br/>
        <a href="${task.no_patch_raw_output}" target=_blank rel="noopener noreferrer">
          NoPatch Raw Output
        </a>
        <br/>
        <a href="${task.with_patch_raw_output}" target=_blank rel="noopener noreferrer">
          WithPatch Raw Output
        </a>`
                  : ''}
      ${task.swarming_logs
                    ? html`<br/>
        <a href="${task.swarming_logs}" target=_blank rel="noopener noreferrer">Swarming Logs</a>`
                    : ''}
    </td>

    <!-- Arguments -->
    <td class=nowrap>
      ${task.benchmark_args
                      ? html`<a href="javascript:;" class=details
          @click=${() => el._showDialog('benchmarkArgs', index)}>
          Benchmark Args
        </a>
        <br/>`
                      : ''}
      ${task.browser_args_no_patch
                        ? html`<a href="javascript:;" class=details
          @click=${() => el._showDialog('browserArgsNoPatch', index)}>
          NoPatch Browser Args
        </a>
        <br/>`
                        : ''}
      ${task.browser_args_with_patch
                          ? html`<a href="javascript:;" class=details
          @click=${() => el._showDialog('browserArgsWithPatch', index)}>
          WithPatch Browser Args
        </a>
        <br/>`
                          : ''}
    </td>

    <!-- Patches -->
    <td>
      ${!isEmptyPatch(task.chromium_patch_gspath)
                            ? html`<a href="${getGSLink(task.chromium_patch_gspath)}"
        target="_blank" rel="noopener noreferrer">Chromium</a>
        <br/>
        `
                            : ''}
      ${!isEmptyPatch(task.blink_patch_gspath)
                              ? html`<a href="${getGSLink(task.blink_patch_gspath)}"
        target="_blank" rel="noopener noreferrer">Blink</a>
        <br/>
        `
                              : ''}
      ${!isEmptyPatch(task.skia_patch_gspath)
                                ? html`<a href="${getGSLink(task.skia_patch_gspath)}"
        target="_blank" rel="noopener noreferrer">Skia</a>
        <br/>
        `
                                : ''}
      ${!isEmptyPatch(task.v8_patch_gspath)
                                  ? html`<a href="${getGSLink(task.v8_patch_gspath)}"
        target="_blank" rel="noopener noreferrer">V8</a>
        <br/>
        `
                                  : ''}
      ${!isEmptyPatch(task.catapult_patch_gspath)
                                    ? html`<a href="${getGSLink(task.catapult_patch_gspath)}"
        target="_blank" rel="noopener noreferrer">Catapult</a>
        <br/>
        `
                                    : ''}
      ${!isEmptyPatch(task.benchmark_patch_gspath)
                                      ? html`<a href="${getGSLink(task.benchmark_patch_gspath)}"
        target="_blank" rel="noopener noreferrer">Telemetry</a>
        <br/>
        `
                                      : ''}
      ${!isEmptyPatch(task.chromium_patch_base_build_gspath)
                                        ? html`<a href="${getGSLink(task.chromium_patch_base_build_gspath)}"
        target="_blank" rel="noopener noreferrer">Chromium(base_build)</a>
        <br/>
        `
                                        : ''}
    </td>

    <!-- Task Repeats -->
    <td>${formatRepeatAfterDays(task.repeat_after_days)}</td>
  </tr>`;


  private static taskDialogTemplate = (task: ChromiumPerfDatastoreTask, index: number) => html`
  <div id=${`benchmarkArgs${index}`} class="dialog-background hidden overlay-themes-sk"
    @click=${hideDialog}>
    <div class="dialog-content surface-themes-sk">
      <pre>${task.benchmark_args}</pre>
    </div>
  </div>
  <div id=${`browserArgsNoPatch${index}`} class="dialog-background hidden overlay-themes-sk"
    @click=${hideDialog}>
    <div class="dialog-content surface-themes-sk">
      <pre>${task.browser_args_no_patch}</pre>
    </div>
  </div>
  <div id=${`browserArgsWithPatch${index}`} class="dialog-background hidden overlay-themes-sk"
    @click=${hideDialog}>
    <div class="dialog-content surface-themes-sk">
      <pre>${task.browser_args_with_patch}</pre>
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
      this._render();
      this._reload().then(() => {
        this._running = false;
      });
    });
  }

  _showDialog(type, index) {
    $$(`#${type}${index}`, this).classList.remove('hidden');
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
    return fetch(`/_/get_chromium_perf_tasks?${fromObject(queryParams)}`,
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
    fetch('/_/delete_chromium_perf_task', { method: 'POST', body: JSON.stringify(params) })
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
    fetch('/_/redo_chromium_perf_task', { method: 'POST', body: JSON.stringify(params) })
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
}

define('chromium-perf-runs-sk', ChromiumPerfRunsSk);
