/**
 * @fileoverview The bulk of the Chromium Analysis Runs History page.
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

const template = (el) => html`
<div>
  <h2>${el._contrainByUser ? 'My ' : ''}Chromium Analysis Runs</h2>
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
      <th>Description</th>
      <th>Results</th>
      <th>Arguments</th>
      <th>Patches</th>
      <th>Task Repeats</th>
    </tr>
    ${el._tasks.map((task, index) => taskRowTemplate(el, task, index))}
  </table>
</div>

${el._tasks.map((task, index) => taskDialogTemplate(task, index))}
<confirm-dialog-sk id=confirm_dialog></confirm-dialog-sk>
<toast-sk id=confirm_toast class=primary-variant-container-themes-sk duration=5000></toast-sk>
`;

const taskRowTemplate = (el, task, index) => html`
<tr>
  <!-- Id col -->
  <td class=nowrap>
    ${task.RawOutput
    ? html`<a href="${task.RawOutput}" target=_blank rel="noopener noreferrer">${task.Id}</a>`
    : html`<span>${task.Id}</span>`}
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
        <td>Benchmark:</td>
        <td>${task.Benchmark}</td>
      </tr>
      <tr>
        <td>Platform:</td>
        <td>${task.Platform}</td>
      </tr>
      <tr>
        <td>RunOnGCE:</td>
        <td>${task.RunOnGCE}</td>
      </tr>
      <tr>
        <td>PageSet:</td>
        <td>
          ${!isEmptyPatch(task.CustomWebpagesGSPath)
    ? html`<a href="${getGSLink(task.CustomWebpagesGSPath)}"
              target=_blank rel="noopener noreferrer">Custom Webpages</a>`
    : task.PageSets}
        </td>
      </tr>
      <tr>
        <td>ParallelRun:</td>
        <td>${task.RunInParallel}</td>
      </tr>
      ${task.ValueColumnName
    ? html`<tr>
          <td class=nowrap>Value Column:</td>
          <td class=nowrap>${task.ValueColumnName}</td>
          </tr>`
    : ''}
      ${task.TaskPriority
    ? html`<tr>
          <td>TaskPriority:</td>
          <td>${task.TaskPriority}</td>
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
      ${task.ApkGsPath
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
      ${task.TelemetryIsolateHash
    ? html`<tr>
          <td>TelemetryIsolateHash:</td>
          <td><a href="https://chrome-isolated.appspot.com/browse?digest=${task.TelemetryIsolateHash}">${task.TelemetryIsolateHash}</a></td>
        </tr>`
    : ''}
    </table>
  </td>

  <!-- Description col -->
  <td>${task.Description}</td>

  <!-- Results col -->
  <td class=nowrap>
    ${task.Failure ? html`<div class=error>Failed</div>` : ''}
    ${!task.TaskDone ? html`<div class=green>Waiting</div>` : ''}
    ${task.RawOutput
    ? html`<a href="${task.RawOutput}" target=_blank rel="noopener noreferrer">
        Output
      </a>`
    : ''}
    ${task.SwarmingLogs
    ? html`<br/>
      <a href="${task.SwarmingLogs}" target=_blank rel="noopener noreferrer">
        Swarming Logs
      </a>`
    : ''}
  </td>

  <!-- Arguments -->
  <td class=nowrap>
    ${task.BenchmarkArgs
    ? html`<a href="javascript:;" class=details
        @click=${() => el._showDialog('benchmarkArgs', index)}>
        Benchmark Args
      </a>
      <br/>`
    : ''}
    ${task.BrowserArgs
    ? html`<a href="javascript:;" class=details
        @click=${() => el._showDialog('browserArgs', index)}>
        Browser Args
      </a>
      <br/>`
    : ''}
    ${task.MatchStdoutTxt
    ? html`<a href="javascript:;" class=details
        @click=${() => el._showDialog('matchStdoutTxt', index)}>
        Match Stdout Text
      </a>
      <br/>`
    : ''}
  </td>

  <!-- Patches -->
  <td>
    ${!isEmptyPatch(task.ChromiumPatchGSPath)
    ? html`<a href="${getGSLink(task.ChromiumPatchGSPath)}"
      target="_blank" rel="noopener noreferrer">Chromium</a>
      <br/>
      `
    : ''}
    ${!isEmptyPatch(task.SkiaPatchGSPath)
    ? html`<a href="${getGSLink(task.SkiaPatchGSPath)}"
      target="_blank" rel="noopener noreferrer">Skia</a>
      <br/>
      `
    : ''}
    ${!isEmptyPatch(task.V8PatchGSPath)
    ? html`<a href="${getGSLink(task.V8PatchGSPath)}"
      target="_blank" rel="noopener noreferrer">V8</a>
      <br/>
      `
    : ''}
    ${!isEmptyPatch(task.CatapultPatchGSPath)
    ? html`<a href="${getGSLink(task.CatapultPatchGSPath)}"
      target="_blank" rel="noopener noreferrer">Catapult</a>
      <br/>
      `
    : ''}
    ${!isEmptyPatch(task.BenchmarkPatchGSPath)
    ? html`<a href="${getGSLink(task.BenchmarkPatchGSPath)}"
      target="_blank" rel="noopener noreferrer">Telemetry</a>
      <br/>
      `
    : ''}
  </td>

  <!-- Task Repeats -->
  <td>${formatRepeatAfterDays(task.RepeatAfterDays)}</td>
</tr>`;


const taskDialogTemplate = (task, index) => html`
<div id=${`benchmarkArgs${index}`} class="dialog-background hidden overlay-themes-sk"
  @click=${hideDialog}>
  <div class="dialog-content surface-themes-sk">
    <pre>${task.BenchmarkArgs}</pre>
  </div>
</div>
<div id=${`browserArgs${index}`} class="dialog-background hidden overlay-themes-sk"
  @click=${hideDialog}>
  <div class="dialog-content surface-themes-sk">
    <pre>${task.BrowserArgs}</pre>
  </div>
</div>
<div id=${`matchStdoutTxt${index}`} class="dialog-background hidden overlay-themes-sk"
  @click=${hideDialog}>
  <div class="dialog-content surface-themes-sk">
    <pre>${task.MatchStdoutTxt}</pre>
  </div>
</div>
<div id=${`apkGsPath${index}`} class="dialog-background hidden overlay-themes-sk"
  @click=${hideDialog}>
  <div class="dialog-content surface-themes-sk">
    <pre>${task.ApkGsPath}</pre>
  </div>
</div>
`;

const hideDialog = (e) => {
  if (e.target.classList.contains('dialog-background')) {
    e.target.classList.add('hidden');
  }
};

define('chromium-analysis-runs-sk', class extends ElementSk {
  constructor() {
    super(template);
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
    return fetch(`/_/get_chromium_analysis_tasks?${fromObject(queryParams)}`,
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
    fetch('/_/delete_chromium_analysis_task', { method: 'POST', body: JSON.stringify(params) })
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
    fetch('/_/redo_chromium_analysis_task', { method: 'POST', body: JSON.stringify(params) })
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
