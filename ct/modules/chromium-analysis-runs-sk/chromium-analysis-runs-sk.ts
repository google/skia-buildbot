/**
 * @fileoverview The bulk of the Chromium Analysis Runs History page.
 */

import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/icon/redo-icon-sk';
import 'elements-sk/icon/cancel-icon-sk';
import 'elements-sk/icon/check-circle-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/toast-sk';
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
  getFormattedTimestamp, getGSLink, isEmptyPatch, formatRepeatAfterDays,
} from '../ctfe_utils';
import {
  ResponsePagination,
  ChromiumAnalysisDatastoreTask,
  RedoTaskRequest,
  DeleteTaskRequest,
  GetTasksResponse,
} from '../json';

function hideDialog(e: Event) {
  const classList = (e.target as HTMLElement).classList;
  if (classList.contains('dialog-background')) {
    classList.add('hidden');
  }
}

export class ChromiumAnalysisRunsSk extends ElementSk {
  private _tasks: ChromiumAnalysisDatastoreTask[] = [];

  private _constrainByUser = false;

  private _constrainByTest = true;

  private _running = false;

  private _pagination: ResponsePagination | null = null;

  constructor() {
    super(ChromiumAnalysisRunsSk.template);
    this._resetPagination();
  }

  private static template = (el: ChromiumAnalysisRunsSk) => html`
<div>
  <h2>${el._constrainByUser ? 'My ' : ''}Chromium Analysis Runs</h2>
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
    <tr>
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
    ${el._tasks.map((task, index) => ChromiumAnalysisRunsSk.taskRowTemplate(el, task, index))}
  </table>
</div>

${el._tasks.map((task, index) => ChromiumAnalysisRunsSk.taskDialogTemplate(task, index))}
<toast-sk id=confirm_toast duration=5000></toast-sk>
`;

  private static taskRowTemplate = (el: ChromiumAnalysisRunsSk, task: ChromiumAnalysisDatastoreTask, index: number) => html`
<tr>
  <!-- Id col -->
  <td class=nowrap>
    ${task.raw_output
    ? html`<a href="${task.raw_output}" target=_blank rel="noopener noreferrer">${task.id}</a>`
    : html`<span>${task.id}</span>`}
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
        <td>RunOnGCE:</td>
        <td>${task.run_on_gce}</td>
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
      ${task.cc_list
            ? html`<tr>
          <td>CC List:</td>
          <td>${task.cc_list}</td>
        </tr>`
            : ''}
      ${task.group_name
              ? html`<tr>
          <td>GroupName:</td>
          <td><a href="https://ct-perf.skia.org/e/?request_type=1">${task.group_name}</a></td>
        </tr>`
              : ''}
      ${task.chromium_hash
                ? html`<tr>
          <td>ChromiumHash:</td>
          <td><a href="https://chromium.googlesource.com/chromium/src/+show/${task.chromium_hash}">${task.chromium_hash}</a></td>
        </tr>`
                : ''}
      ${task.apk_gspath
                  ? html`<tr>
          <td>ApkGsPath:</td>
          <td>
            <a href="javascript:;" class=details
              @click=${() => el._showDialog('apkGsPath', index)}>
              Display Path
            </a>
          </td>
        </tr>`
                  : ''}
      ${task.chrome_build_gs_path
                    ? html`<tr>
          <td>ChromeBuildGsPath:</td>
          <td>
            <a href="javascript:;" class=details
              @click=${() => el._showDialog('chromeBuildGsPath', index)}>
              Display Path
            </a>
          </td>
        </tr>`
                    : ''}
      ${task.telemetry_isolate_hash
                      ? html`<tr>
          <td>TelemetryCASHash:</td>
          <td><a href="https://cas-viewer.appspot.com/projects/chrome-swarming/instances/default_instance/blobs/${task.telemetry_isolate_hash}/tree">${task.telemetry_isolate_hash}</a></td>
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
    ${task.raw_output
                        ? html`<a href="${task.raw_output}" target=_blank rel="noopener noreferrer">
        Output
      </a>`
                        : ''}
    ${task.swarming_logs
                          ? html`<br/>
      <a href="${task.swarming_logs}" target=_blank rel="noopener noreferrer">
        Swarming Logs
      </a>`
                          : ''}
  </td>

  <!-- Arguments -->
  <td class=nowrap>
    ${task.gn_args
                            ? html`<a href="javascript:;" class=details
        @click=${() => el._showDialog('gnArgs', index)}>
        GN Args
      </a>
      <br/>`
                            : ''}
    ${task.benchmark_args
                            ? html`<a href="javascript:;" class=details
        @click=${() => el._showDialog('benchmarkArgs', index)}>
        Benchmark Args
      </a>
      <br/>`
                            : ''}
    ${task.browser_args
                              ? html`<a href="javascript:;" class=details
        @click=${() => el._showDialog('browserArgs', index)}>
        Browser Args
      </a>
      <br/>`
                              : ''}
    ${task.match_stdout_txt
                                ? html`<a href="javascript:;" class=details
        @click=${() => el._showDialog('matchStdoutTxt', index)}>
        Match Stdout Text
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
  </td>

  <!-- Task Repeats -->
  <td>${formatRepeatAfterDays(task.repeat_after_days)}</td>
</tr>`;

  private static taskDialogTemplate = (task: ChromiumAnalysisDatastoreTask, index: number) => html`
<div id=${`gnArgs${index}`} class="dialog-background hidden overlay-themes-sk"
  @click=${hideDialog}>
  <div class="dialog-content surface-themes-sk">
    <pre>${task.gn_args}</pre>
  </div>
</div>
<div id=${`benchmarkArgs${index}`} class="dialog-background hidden overlay-themes-sk"
  @click=${hideDialog}>
  <div class="dialog-content surface-themes-sk">
    <pre>${task.benchmark_args}</pre>
  </div>
</div>
<div id=${`browserArgs${index}`} class="dialog-background hidden overlay-themes-sk"
  @click=${hideDialog}>
  <div class="dialog-content surface-themes-sk">
    <pre>${task.browser_args}</pre>
  </div>
</div>
<div id=${`matchStdoutTxt${index}`} class="dialog-background hidden overlay-themes-sk"
  @click=${hideDialog}>
  <div class="dialog-content surface-themes-sk">
    <pre>${task.match_stdout_txt}</pre>
  </div>
</div>
<div id=${`apkGsPath${index}`} class="dialog-background hidden overlay-themes-sk"
  @click=${hideDialog}>
  <div class="dialog-content surface-themes-sk">
    <pre>${task.apk_gspath}</pre>
  </div>
</div>
<div id=${`chromeBuildGsPath${index}`} class="dialog-background hidden overlay-themes-sk"
  @click=${hideDialog}>
  <div class="dialog-content surface-themes-sk">
    <pre>${task.chrome_build_gs_path}</pre>
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

  _showDialog(type: string, index: number): void {
  $$(`#${type}${index}`, this)!.classList.remove('hidden');
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
    return fetch(`/_/get_chromium_analysis_tasks?${fromObject(queryParams)}`,
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
    fetch('/_/delete_chromium_analysis_task', { method: 'POST', body: JSON.stringify(req) })
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
    fetch('/_/redo_chromium_analysis_task', { method: 'POST', body: JSON.stringify(req) })
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

define('chromium-analysis-runs-sk', ChromiumAnalysisRunsSk);
