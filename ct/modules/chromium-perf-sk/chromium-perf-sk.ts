/**
 * @fileoverview The bulk of the Performance page of CT.
 */

import '../../../elements-sk/modules/icons/delete-icon-sk';
import '../../../elements-sk/modules/icons/cancel-icon-sk';
import '../../../elements-sk/modules/icons/check-circle-icon-sk';
import '../../../elements-sk/modules/icons/help-icon-sk';
import '../../../elements-sk/modules/toast-sk';
import '../suggest-input-sk';
import '../input-sk';
import '../patch-sk';
import '../pageset-selector-sk';
import '../task-repeater-sk';
import '../task-priority-sk';

import { html } from 'lit/html.js';
import { $$, $ } from '../../../infra-sk/modules/dom';
import { define } from '../../../elements-sk/modules/define';
import '../../../elements-sk/modules/select-sk';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';

import { SelectSk } from '../../../elements-sk/modules/select-sk/select-sk';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import { InputSk } from '../input-sk/input-sk';
import { PagesetSelectorSk } from '../pageset-selector-sk/pageset-selector-sk';
import { PatchSk } from '../patch-sk/patch-sk';
import { TaskPrioritySk } from '../task-priority-sk/task-priority-sk';
import { TaskRepeaterSk } from '../task-repeater-sk/task-repeater-sk';
import { ChromiumPerfAddTaskVars, EditTaskRequest } from '../json';
import {
  combineClDescriptions,
  missingLiveSitesWithCustomWebpages,
  moreThanThreeActiveTasksChecker,
  fetchBenchmarksAndPlatforms,
} from '../ctfe_utils';

// Chromium perf doesn't support 1M and 100K pagesets.
const unsupportedPageSets = ['All', '100k', 'Mobile100k'];

export class ChromiumPerfSk extends ElementSk {
  _platforms: [string, unknown][] = [];

  private _benchmarksToDocs: Record<string, string> = {};

  private _benchmarks: string[] = [];

  private _triggeringTask: boolean = false;

  private _moreThanThreeActiveTasks = moreThanThreeActiveTasksChecker();

  // Variables that represent UI elements.
  private benchmark!: InputSk;

  private platform!: SelectSk;

  private pageSets!: PagesetSelectorSk;

  private runInParallel!: SelectSk;

  private gnArgs!: InputSk;

  private benchmarkArgs!: InputSk;

  private browserArgsNoPatch!: InputSk;

  private browserArgsWithPatch!: InputSk;

  private valueColumnName!: InputSk;

  private description!: InputSk;

  private chromiumPatch!: PatchSk;

  private skiaPatch!: PatchSk;

  private v8Patch!: PatchSk;

  private catapultPatch!: PatchSk;

  private chromiumBaseBuildPatch!: PatchSk;

  private chromiumHash!: InputSk;

  private repeatAfterDays!: TaskRepeaterSk;

  private taskPriority!: TaskPrioritySk;

  private ccList!: InputSk;

  private groupName!: InputSk;

  constructor() {
    super(ChromiumPerfSk.template);
  }

  private static template = (el: ChromiumPerfSk) => html`
    <table class="options">
      <tr>
        <td>Benchmark Name</td>
        <td>
          <suggest-input-sk
            id="benchmark_name"
            .options=${el._benchmarks}
            .label=${'Hit <enter> at end if entering custom benchmark'}
            accept-custom-value
            @value-changed=${el._refreshBenchmarkDoc}></suggest-input-sk>
          <div>
            <a
              hidden
              id="benchmark_doc"
              href="#"
              target="_blank"
              rel="noopener noreferrer">
              Documentation
            </a>
          </div>
        </td>
      </tr>
      <tr>
        <td>Target Platform</td>
        <td>
          <select-sk
            id="platform_selector"
            @selection-changed=${el._platformChanged}>
            ${el._platforms.map(
              (p, i) => html`<div ?selected=${i === 1}>${p[1]}</div>`
            )}
          </select-sk>
        </td>
      </tr>
      <tr>
        <td>PageSets Type</td>
        <td>
          <pageset-selector-sk
            id="pageset_selector"
            .hideKeys=${unsupportedPageSets}>
          </pageset-selector-sk>
        </td>
      </tr>
      <tr>
        <td>
          Run in Parallel<br />
          Read about the trade-offs
          <a
            href="https://docs.google.com/document/d/1GhqosQcwsy6F-eBAmFn_ITDF7_Iv_rY9FhCKwAnk9qQ/edit?pli=1#heading=h.xz46aihphb8z"
            >here</a
          >
        </td>
        <td>
          <select-sk id="run_in_parallel">
            <div>True</div>
            <div selected>False</div>
          </select-sk>
        </td>
      </tr>
      <tr>
        <td>GN Arguments for Build</td>
        <td>
          <input-sk
            value="is_debug=false treat_warnings_as_errors=false dcheck_always_on=false is_official_build=true enable_nacl=false symbol_level=1"
            id="gn_args"
            class="long-field"></input-sk>
          <span class="smaller-font"
            ><b>Note:</b> Android runs will automatically include
            target_os="android".</span
          ><br />
        </td>
      </tr>
      <tr>
        <td>Benchmark Arguments</td>
        <td>
          <input-sk
            value="--output-format=csv --pageset-repeat=1 --skip-typ-expectations-tags-validation --legacy-json-trace-format"
            id="benchmark_args"
            class="long-field"></input-sk>
          <span class="smaller-font"
            ><b>Note:</b> Change the --pageset-repeat value if you would like
            lower/higher repeats of each web page. 1 is the default.</span
          ><br />
          <span class="smaller-font"
            ><b>Note:</b> Use --run-benchmark-timeout=[secs] to specify the
            timeout of the run_benchmark script. 300 is the default.</span
          ><br />
          <span class="smaller-font"
            ><b>Note:</b> Use --max-pages-per-bot=[num] to specify the number of
            pages to run per bot. 100 is the default.</span
          >
        </td>
      </tr>
      <tr>
        <td>Browser Arguments (nopatch run)</td>
        <td>
          <input-sk
            value=""
            id="browser_args_nopatch"
            class="long-field"></input-sk>
        </td>
      </tr>
      <tr>
        <td>Browser Arguments (withpatch run)</td>
        <td>
          <input-sk
            value=""
            id="browser_args_withpatch"
            class="long-field"></input-sk>
        </td>
      </tr>
      <tr>
        <td>Field Value Column Name</td>
        <td>
          <input-sk
            value="avg"
            id="value_column_name"
            class="medium-field"></input-sk>
          <span class="smaller-font"
            >Which column's entries to use as field values.</span
          >
        </td>
      </tr>
      <tr>
        <td>
          Chromium Git patch (optional)<br />
          Applied to Chromium ToT<br />
          or to the hash specified below.
        </td>
        <td>
          <patch-sk
            id="chromium_patch"
            patchType="chromium"
            @cl-description-changed=${el._patchChanged}>
          </patch-sk>
        </td>
      </tr>
      <tr>
        <td>
          Skia Git patch (optional)<br />
          Applied to Skia Rev in
          <a
            href="https://chromium.googlesource.com/chromium/src/+show/HEAD/DEPS"
            >DEPS</a
          >
        </td>
        <td>
          <patch-sk
            id="skia_patch"
            patchType="skia"
            @cl-description-changed=${el._patchChanged}>
          </patch-sk>
        </td>
      </tr>
      <tr>
        <td>
          V8 Git patch (optional)<br />
          Applied to V8 Rev in
          <a
            href="https://chromium.googlesource.com/chromium/src/+show/HEAD/DEPS"
            >DEPS</a
          >
        </td>
        <td>
          <patch-sk
            id="v8_patch"
            patchType="v8"
            @cl-description-changed=${el._patchChanged}>
          </patch-sk>
        </td>
      </tr>
      <tr>
        <td>
          Catapult Git patch (optional)<br />
          Applied to Catapult Rev in
          <a
            href="https://chromium.googlesource.com/chromium/src/+show/HEAD/DEPS"
            >DEPS</a
          >
        </td>
        <td>
          <patch-sk
            id="catapult_patch"
            patchType="catapult"
            @cl-description-changed=${el._patchChanged}>
          </patch-sk>
        </td>
      </tr>
      <tr>
        <td>
          Chromium Git metrics patch (optional)<br />
          Applied to Chromium ToT<br />
          or to the hash specified below.<br />
          Used to create the base build (See
          <a href="http://skbug.com/9029">skbug/9029</a>)
        </td>
        <td>
          <patch-sk
            id="chromium_patch_base_build"
            patchType="chromium"
            @cl-description-changed=${el._patchChanged}>
          </patch-sk>
        </td>
      </tr>
      <tr>
        <td>Chromium hash to sync to (optional)<br /></td>
        <td>
          <input-sk value="" id="chromium_hash" class="long-field"></input-sk>
        </td>
      </tr>
      <tr>
        <td>Repeat this task</td>
        <td>
          <task-repeater-sk id="repeat_after_days"></task-repeater-sk>
        </td>
      </tr>
      <tr>
        <td>Task Priority</td>
        <td>
          <task-priority-sk id="task_priority"></task-priority-sk>
        </td>
      </tr>
      <tr>
        <td>
          Notifications CC list (optional)<br />
          Email will be sent by ct@skia.org
        </td>
        <td>
          <input-sk
            value=""
            id="cc_list"
            label="email1,email2,email3"
            class="long-field"></input-sk>
        </td>
      </tr>
      <tr>
        <td>
          Group name (optional)<br />
          Will be used to track runs
        </td>
        <td>
          <input-sk value="" id="group_name" class="long-field"></input-sk>
        </td>
      </tr>
      <tr>
        <td>Description</td>
        <td>
          <input-sk
            value=""
            id="description"
            label="Description is required"
            class="long-field"></input-sk>
        </td>
      </tr>
      <tr>
        <td colspan="2" class="center">
          <div class="triggering-spinner">
            <spinner-sk
              .active=${el._triggeringTask}
              alt="Trigger task"></spinner-sk>
          </div>
          <button
            id="submit"
            ?disabled=${el._triggeringTask}
            @click=${el._validateTask}>
            Queue Task
          </button>
        </td>
      </tr>
      <tr>
        <td colspan="2" class="center">
          <button id="view_history" @click=${el._gotoRunsHistory}>
            View runs history
          </button>
        </td>
      </tr>
    </table>
  `;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();

    // Assign to all member variables that represent UI elements.
    this.benchmark = $$<InputSk>('#benchmark_name', this)!;
    this.platform = $$<SelectSk>('#platform_selector', this)!;
    this.pageSets = $$<PagesetSelectorSk>('#pageset_selector', this)!;
    this.runInParallel = $$<SelectSk>('#run_in_parallel', this)!;
    this.gnArgs = $$<InputSk>('#gn_args', this)!;
    this.benchmarkArgs = $$<InputSk>('#benchmark_args', this)!;
    this.browserArgsNoPatch = $$<InputSk>('#browser_args_nopatch', this)!;
    this.browserArgsWithPatch = $$<InputSk>('#browser_args_withpatch', this)!;
    this.valueColumnName = $$<InputSk>('#value_column_name', this)!;
    this.description = $$<InputSk>('#description', this)!;
    this.chromiumPatch = $$<PatchSk>('#chromium_patch', this)!;
    this.skiaPatch = $$<PatchSk>('#skia_patch', this)!;
    this.v8Patch = $$<PatchSk>('#v8_patch', this)!;
    this.catapultPatch = $$<PatchSk>('#catapult_patch', this)!;
    this.chromiumBaseBuildPatch = $$<PatchSk>(
      '#chromium_patch_base_build',
      this
    )!;
    this.chromiumHash = $$<InputSk>('#chromium_hash', this)!;
    this.repeatAfterDays = $$<TaskRepeaterSk>('#repeat_after_days', this)!;
    this.taskPriority = $$<TaskPrioritySk>('#task_priority', this)!;
    this.ccList = $$<InputSk>('#cc_list', this)!;
    this.groupName = $$<InputSk>('#group_name', this)!;

    // If template_id is specified then load the template.
    const params = new URLSearchParams(window.location.search);
    const template_id = params.get('template_id');
    if (template_id) {
      this.handleTemplateID(template_id);
    }

    fetchBenchmarksAndPlatforms((json) => {
      this._benchmarksToDocs = json.benchmarks || {};
      this._benchmarks = Object.keys(json.benchmarks || {});
      // { 'p1' : 'p1Desc', ... } -> [[p1, p1Desc], ...]
      // Allows rendering descriptions in the select-sk, and converting the
      // integer selection to platform name easily.
      this._platforms = Object.entries(json.platforms || {});
      this._render();
    });
  }

  handleTemplateID(template_id: string): void {
    this.dispatchEvent(new CustomEvent('begin-task', { bubbles: true }));
    const req: EditTaskRequest = { id: +template_id };
    fetch('/_/edit_chromium_perf_task', {
      method: 'POST',
      body: JSON.stringify(req),
    })
      .then(jsonOrThrow)
      .then((json: ChromiumPerfAddTaskVars) => {
        // Populate all fields from the EditTaskRequest.
        this.benchmark.value = json.benchmark;
        // Find the index of the platform and set it.
        Object.keys(this._platforms).forEach((i) => {
          if (this._platforms[+i][0] === json.platform) {
            this.platform.selection = i;
          }
        });
        // Set the page set and custom webpages if specified.
        this.pageSets.selected = json.page_sets;
        if (json.custom_webpages) {
          this.pageSets.customPages = json.custom_webpages;
          this.pageSets.expandTextArea();
        }

        this.runInParallel.selection = json.run_in_parallel === 'true' ? 0 : 1;
        if (json.gn_args) {
          this.gnArgs.value = json.gn_args;
        }
        this.benchmarkArgs.value = json.benchmark_args;
        this.browserArgsNoPatch.value = json.browser_args_nopatch;
        this.browserArgsWithPatch.value = json.browser_args_withpatch;
        this.valueColumnName.value = json.value_column_name;

        // Patches.
        if (json.chromium_patch) {
          this.chromiumPatch.patch = json.chromium_patch;
          this.chromiumPatch.expandTextArea();
        }
        if (json.skia_patch) {
          this.skiaPatch.patch = json.skia_patch;
          this.skiaPatch.expandTextArea();
        }
        if (json.v8_patch) {
          this.v8Patch.patch = json.v8_patch;
          this.v8Patch.expandTextArea();
        }
        if (json.catapult_patch) {
          this.catapultPatch.patch = json.catapult_patch;
          this.catapultPatch.expandTextArea();
        }
        if (json.chromium_patch_base_build) {
          this.chromiumBaseBuildPatch.patch = json.chromium_patch_base_build;
          this.chromiumBaseBuildPatch.expandTextArea();
        }

        this.chromiumHash.value = json.chromium_hash;
        this.repeatAfterDays.frequency = json.repeat_after_days;
        this.taskPriority.priority = json.task_priority;
        if (json.cc_list) {
          this.ccList.value = json.cc_list.join(',');
        }
        this.groupName.value = json.group_name;
        this.description.value = json.desc;

        // Focus and then blur the benchmark name so that we go back to the
        // top of the page.
        this.benchmark.querySelector('input')!.focus();
        this.benchmark.querySelector('input')!.blur();
      })
      .catch(errorMessage)
      .finally(() => {
        this._render();
        this.dispatchEvent(new CustomEvent('end-task', { bubbles: true }));
      });
  }

  _refreshBenchmarkDoc(e: CustomEvent): void {
    const benchmarkName = e.detail.value;
    const docElement = $$('#benchmark_doc', this) as HTMLAnchorElement;
    if (benchmarkName && this._benchmarksToDocs[benchmarkName]) {
      docElement.hidden = false;
      docElement.href = this._benchmarksToDocs[benchmarkName];
    } else {
      docElement.hidden = true;
      docElement.href = '#';
    }
  }

  _platformChanged(e: CustomEvent): void {
    if (e.detail.selection === 0) {
      this.pageSets.selected = 'Mobile10k';
    } else {
      this.pageSets.selected = '10k';
    }
  }

  _patchChanged(): void {
    this.description.value = combineClDescriptions(
      $('patch-sk', this).map((patch) => (patch as PatchSk).clDescription)
    );
  }

  _validateTask(): void {
    if (!$('patch-sk', this).every((patch) => (patch as PatchSk).validate())) {
      return;
    }
    if (!this.description.value) {
      errorMessage('Please specify a description');
      this.description.focus();
      return;
    }
    if (!this.benchmark.value) {
      errorMessage('Please specify a benchmark');
      this.benchmark.focus();
      return;
    }
    if (
      missingLiveSitesWithCustomWebpages(
        this.pageSets.customPages,
        this.benchmarkArgs.value
      )
    ) {
      this.benchmarkArgs.focus();
      return;
    }
    if (this._moreThanThreeActiveTasks()) {
      return;
    }
    const confirmed = window.confirm('Proceed with queueing task?');
    if (confirmed) {
      this._queueTask();
    }
  }

  _queueTask(): void {
    this._triggeringTask = true;
    const params = {} as ChromiumPerfAddTaskVars;
    params.benchmark = this.benchmark.value;
    params.platform = this._platforms[+this.platform.selection!][0];
    params.page_sets = this.pageSets.selected;
    params.custom_webpages = this.pageSets.customPages;
    params.repeat_runs = this._getRepeatValue();
    params.run_in_parallel = ['True', 'False'][+this.runInParallel.selection!];
    params.gn_args = this.gnArgs.value;
    params.benchmark_args = this.benchmarkArgs.value;
    params.browser_args_nopatch = this.browserArgsNoPatch.value;
    params.browser_args_withpatch = this.browserArgsWithPatch.value;
    params.value_column_name = this.valueColumnName.value;
    params.desc = this.description.value;
    params.chromium_patch = this.chromiumPatch.patch;
    params.skia_patch = this.skiaPatch.patch;
    params.v8_patch = this.v8Patch.patch;
    params.catapult_patch = this.catapultPatch.patch;
    params.chromium_patch_base_build = this.chromiumBaseBuildPatch.patch;
    params.chromium_hash = this.chromiumHash.value;
    params.repeat_after_days = this.repeatAfterDays.frequency;
    params.task_priority = this.taskPriority.priority;
    // Run on GCE if it is Windows. This will change in the future if we
    // get bare-metal Win machines.
    params.run_on_gce = (params.platform === 'Windows').toString();
    if (this.ccList.value) {
      params.cc_list = this.ccList.value.split(',');
    }
    if (this.groupName.value) {
      params.group_name = this.groupName.value;
    }

    fetch('/_/add_chromium_perf_task', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(params),
    })
      .then(() => this._gotoRunsHistory())
      .catch((e) => {
        this._triggeringTask = false;
        errorMessage(e);
      });
  }

  _getRepeatValue(): string {
    // If "--pageset-repeat" is specified in benchmark args then use that
    // value else use "1".
    const rx = /--pageset-repeat[ =](\d+)/gm;
    const m = rx.exec(this.benchmarkArgs.value);
    if (m) {
      return m[1];
    }
    return '1';
  }

  _gotoRunsHistory(): void {
    window.location.href = '/chromium_perf_runs/';
  }
}

define('chromium-perf-sk', ChromiumPerfSk);
