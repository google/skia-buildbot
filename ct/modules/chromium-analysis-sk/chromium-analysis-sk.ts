/**
 * @fileoverview The bulk of the Analysis page of CT.
 */

import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/icon/cancel-icon-sk';
import 'elements-sk/icon/check-circle-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/toast-sk';
import '../suggest-input-sk';
import '../input-sk';
import '../patch-sk';
import '../pageset-selector-sk';
import '../task-repeater-sk';
import '../task-priority-sk';

import { $$, $ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import 'elements-sk/select-sk';
import { errorMessage } from 'elements-sk/errorMessage';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';

import { SelectSk } from 'elements-sk/select-sk/select-sk';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import { InputSk } from '../input-sk/input-sk';
import { PagesetSelectorSk } from '../pageset-selector-sk/pageset-selector-sk';
import { PatchSk } from '../patch-sk/patch-sk';
import { TaskPrioritySk } from '../task-priority-sk/task-priority-sk';
import { TaskRepeaterSk } from '../task-repeater-sk/task-repeater-sk';
import {
  ChromiumAnalysisAddTaskVars,
  EditTaskRequest,
} from '../json';
import {
  combineClDescriptions,
  missingLiveSitesWithCustomWebpages,
  moreThanThreeActiveTasksChecker,
  fetchBenchmarksAndPlatforms,
} from '../ctfe_utils';

// Chromium analysis doesn't support 1M pageset, and only Linux supports 100k.
const unsupportedPageSets = ['All', '100k', 'Mobile100k'];
const unsupportedPageSetsLinux = ['All'];

export class ChromiumAnalysisSk extends ElementSk {
  _platforms: [string, unknown][] = [];

  private _benchmarksToDocs: Record<string, string> = {};

  private _benchmarks: string[] = [];

  private _triggeringTask: boolean = false;

  private _unsupportedPageSets: string[] = unsupportedPageSetsLinux;

  private _moreThanThreeActiveTasks = moreThanThreeActiveTasksChecker();

  // Variables that represent UI elements.
  private benchmark!: InputSk;

  private platform!: SelectSk;

  private pageSets!: PagesetSelectorSk;

  private runOnGCE!: SelectSk;

  private matchStdoutTxt!: InputSk;

  private apkGsPath!: InputSk;

  private chromeBuildGsPath!: InputSk;

  private telemetryIsolateHash!: InputSk;

  private runInParallel!: SelectSk;

  private gnArgs!: InputSk;

  private benchmarkArgs!: InputSk;

  private browserArgs!: InputSk;

  private valueColumnName!: InputSk;

  private description!: InputSk;

  private chromiumPatch!: PatchSk;

  private skiaPatch!: PatchSk;

  private v8Patch!: PatchSk;

  private catapultPatch!: PatchSk;

  private chromiumHash!: InputSk;

  private repeatAfterDays!: TaskRepeaterSk;

  private taskPriority!: TaskPrioritySk;

  private ccList!: InputSk;

  private groupName!: InputSk;

  constructor() {
    super(ChromiumAnalysisSk.template);
  }

  private static template = (el: ChromiumAnalysisSk) => html`

<table class=options>
  <tr>
    <td>Benchmark Name</td>
    <td>
      <suggest-input-sk
        id=benchmark_name
        .options=${el._benchmarks}
        .label=${'Hit <enter> at end if entering custom benchmark'}
        accept-custom-value
        @value-changed=${el._benchmarkChanged}
      ></suggest-input-sk>
      <div>
        <a hidden id=benchmark_doc href=#
        target=_blank rel="noopener noreferrer">
          Documentation
        </a>
      </div>
    </td>
  </tr>
  <tr>
    <td>Target Platform</td>
    <td>
      <select-sk id=platform_selector @selection-changed=${el._platformChanged}>
        ${el._platforms.map((p, i) => (html`<div ?selected=${i === 1}>${p[1]}</div>`))}
      </select-sk>
    </td>
  </tr>
  <tr>
    <td>
      Run on GCE
    </td>
    <td>
      <select-sk id=run_on_gce>
        <div selected id=gce_true>True</div>
        <div id=gce_false>False</div>
      </select-sk>
    </td>
  </tr>
  <tr>
    <td>PageSets Type</td>
    <td>
      <pageset-selector-sk id=pageset_selector></pageset-selector-sk>
    </td>
  </tr>
  <tr>
    <td>
      Run in Parallel<br/>
      Read about the trade-offs <a href="https://docs.google.com/document/d/1GhqosQcwsy6F-eBAmFn_ITDF7_Iv_rY9FhCKwAnk9qQ/edit?pli=1#heading=h.xz46aihphb8z">here</a>
    </td>
    <td>
      <select-sk id=run_in_parallel @selection-changed=${el._updatePageSets}>
        <div selected>True</div>
        <div>False</div>
      </select-sk>
    </td>
  </tr>
  <tr>
    <td>Look for text in stdout</td>
    <td>
      <input-sk value="" id=match_stdout_txt class=long-field></input-sk>
      <span class=smaller-font><b>Note:</b> All lines that contain this field in stdout will show up under CT_stdout_lines in the output CSV.</span><br/>
      <span class=smaller-font><b>Note:</b> The count of non-overlapping exact matches of this field in stdout will show up under CT_stdout_count in the output CSV.</span><br/>
    </td>
  </tr>
  <tr>
    <td>GN Arguments for Build</td>
    <td>
      <input-sk value="is_debug=false treat_warnings_as_errors=false dcheck_always_on=false is_official_build=true enable_nacl=false symbol_level=1" id=gn_args class=long-field></input-sk>
      <span class=smaller-font><b>Note:</b> Android runs will automatically include target_os=\"android\".</span><br/>
    </td>
  </tr>
  <tr>
    <td>Benchmark Arguments</td>
    <td>
      <input-sk value="--output-format=csv --skip-typ-expectations-tags-validation --legacy-json-trace-format" id=benchmark_args class=long-field></input-sk>
      <span class=smaller-font><b>Note:</b> Use --num-analysis-retries=[num] to specify how many times run_benchmark should be retried. 2 is the default. 0 calls run_benchmark once.</span><br/>
      <span class=smaller-font><b>Note:</b> Use --run-benchmark-timeout=[secs] to specify the timeout of the run_benchmark script. 300 is the default.</span><br/>
      <span class=smaller-font><b>Note:</b> Use --max-pages-per-bot=[num] to specify the number of pages to run per bot. 100 is the default.</span>
    </td>
  </tr>
  <tr>
    <td>Browser Arguments</td>
    <td>
      <input-sk value="" id=browser_args class=long-field></input-sk>
    </td>
  </tr>
  <tr>
    <td>Field Value Column Name</td>
    <td>
      <input-sk value="avg" id=value_column_name class="medium-field"></input-sk>
      <span class=smaller-font>Which column's entries to use as field values.</span>
    </td>
  </tr>
  <tr>
    <td>
      Chromium Git patch (optional)<br/>
      Applied to Chromium ToT<br/>
      or to the hash specified below.
    </td>
    <td>
      <patch-sk id=chromium_patch
                patchType=chromium
                @cl-description-changed=${el._patchChanged}>
      </patch-sk>
    </td>
  </tr>
  <tr>
    <td>
      Custom APK location for Android<br/>
      (optional)<br/> (See
      <a href="https://bugs.chromium.org/p/skia/issues/detail?id=9805">skbug/9805</a>)
    </td>
    <td>
      <input-sk value="" id=apk_gs_path label="Eg: gs://chrome-unsigned/android-B0urB0N/73.0.3655.0/arm_64/ChromeModern.apk" class=long-field></input-sk>
    </td>
  </tr>
  <tr>
    <td>
      Custom Chrome build zip location<br/>
      for non-Android runs (optional)<br/> (See
      <a href="https://bugs.chromium.org/p/skia/issues/detail?id=11395">skbug/11395</a>)
    </td>
    <td>
      <input-sk value="" id=chrome_build_gs_path label="Eg: gs://chromium-browser-snapshots/Linux_x64/805044/chrome-linux.zip" class=long-field></input-sk>
    </td>
  </tr>
  <tr>
    <td>
      Telemetry CAS Hash (optional)<br/> (See
      <a href="https://bugs.chromium.org/p/skia/issues/detail?id=9853#c11">skbug/9853</a>)
    </td>
    <td>
      <input-sk value="" label="Eg: 704a0dfa6e4d24599dc362fb8db5ffb918d806959ace1e75066ea6ed4f55a50a/652" id=telemetry_isolate_hash class=long-field></input-sk>
    </td>
  </tr>
  <tr>
    <td>Chromium hash to sync to (optional)<br/></td>
    <td>
      <input-sk value="" id=chromium_hash class=long-field></input-sk>
    </td>
  </tr>
  <tr>
    <td>
      Skia Git patch (optional)<br/>
      Applied to Skia Rev in <a href="https://chromium.googlesource.com/chromium/src/+show/HEAD/DEPS">DEPS</a>
    </td>
    <td>
      <patch-sk id=skia_patch
                patchType=skia
                @cl-description-changed=${el._patchChanged}>
      </patch-sk>
    </td>
  </tr>
  <tr>
    <td>
      V8 Git patch (optional)<br/>
      Applied to V8 Rev in <a href="https://chromium.googlesource.com/chromium/src/+show/HEAD/DEPS">DEPS</a>
    </td>
    <td>
      <patch-sk id=v8_patch
                patchType=v8
                @cl-description-changed=${el._patchChanged}>
      </patch-sk>
    </td>
  </tr>
  <tr>
    <td>
      Catapult Git patch (optional)<br/>
      Applied to Catapult Rev in <a href="https://chromium.googlesource.com/chromium/src/+show/HEAD/DEPS">DEPS</a>
    </td>
    <td>
      <patch-sk id=catapult_patch
                patchType=catapult
                @cl-description-changed=${el._patchChanged}>
      </patch-sk>
    </td>
  </tr>
  <tr>
    <td>Repeat this task</td>
    <td>
      <task-repeater-sk id=repeat_after_days></task-repeater-sk>
    </td>
  </tr>
  <tr>
    <td>Task Priority</td>
    <td>
      <task-priority-sk id=task_priority></task-priority-sk>
    </td>
  </tr>
  <tr>
    <td>
      Notifications CC list (optional)<br/>
      Email will be sent by ct@skia.org
    </td>
    <td>
      <input-sk value="" id=cc_list label="email1,email2,email3" class=long-field></input-sk>
    </td>
  </tr>
  <tr>
    <td>
      Group name (optional)<br/>
      Will be used to track runs
    </td>
    <td>
      <input-sk value="" id=group_name class=long-field></input-sk>
    </td>
  </tr>
  <tr>
    <td>Description</td>
    <td>
      <input-sk value="" id=description label="Description is required" class=long-field></input-sk>
    </td>
  </tr>
  <tr>
    <td colspan="2" class="center">
      <div class="triggering-spinner">
        <spinner-sk .active=${el._triggeringTask} alt="Trigger task"></spinner-sk>
      </div>
      <button id=submit ?disabled=${el._triggeringTask} @click=${el._validateTask}>Queue Task</button>
    </td>
  </tr>
  <tr>
    <td colspan=2 class=center>
      <button id=view_history @click=${el._gotoRunsHistory}>View runs history</button>
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
    this.runOnGCE = $$<SelectSk>('#run_on_gce', this)!;
    this.matchStdoutTxt = $$<InputSk>('#match_stdout_txt', this)!;
    this.apkGsPath = $$<InputSk>('#apk_gs_path', this)!;
    this.chromeBuildGsPath = $$<InputSk>('#chrome_build_gs_path', this)!;
    this.telemetryIsolateHash = $$<InputSk>('#telemetry_isolate_hash', this)!;
    this.runInParallel = $$<SelectSk>('#run_in_parallel', this)!;
    this.gnArgs = $$<InputSk>('#gn_args', this)!;
    this.benchmarkArgs = $$<InputSk>('#benchmark_args', this)!;
    this.browserArgs = $$<InputSk>('#browser_args', this)!;
    this.valueColumnName = $$<InputSk>('#value_column_name', this)!;
    this.description = $$<InputSk>('#description', this)!;
    this.chromiumPatch = $$<PatchSk>('#chromium_patch', this)!;
    this.skiaPatch = $$<PatchSk>('#skia_patch', this)!;
    this.v8Patch = $$<PatchSk>('#v8_patch', this)!;
    this.catapultPatch = $$<PatchSk>('#catapult_patch', this)!;
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
      // Do this after the template is rendered, or else it fails, and don't
      // inline a child 'selected' attribute since it won't rationalize in
      // select-sk until later via the mutationObserver.
      this.platform.selection = 1;
      // This gets the defaults in a valid state.
      this._platformChanged();
    });
  }

  handleTemplateID(template_id: string): void {
    this.dispatchEvent(new CustomEvent('begin-task', { bubbles: true }));
    const req: EditTaskRequest = { id: +template_id };
    fetch('/_/edit_chromium_analysis_task', { method: 'POST', body: JSON.stringify(req) })
      .then(jsonOrThrow)
      .then((json: ChromiumAnalysisAddTaskVars) => {
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

        this.runOnGCE.selection = json.run_on_gce ? 0 : 1;
        this.matchStdoutTxt.value = json.match_stdout_txt;
        this.apkGsPath.value = json.apk_gs_path;
        this.chromeBuildGsPath.value = json.chrome_build_gs_path;
        this.telemetryIsolateHash.value = json.telemetry_isolate_hash;
        this.runInParallel.selection = json.run_in_parallel ? 0 : 1;
        if (json.gn_args) {
          this.gnArgs.value = json.gn_args;
        }
        this.benchmarkArgs.value = json.benchmark_args;
        this.browserArgs.value = json.browser_args;
        this.valueColumnName.value = json.value_column_name;
        this.description.value = json.desc;

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

        this.chromiumHash.value = json.chromium_hash;
        this.repeatAfterDays.frequency = json.repeat_after_days;
        this.taskPriority.priority = json.task_priority;
        if (json.cc_list) {
          this.ccList.value = json.cc_list.join(',');
        }
        this.groupName.value = json.group_name;

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

  _benchmarkChanged(e: CustomEvent): void {
    const benchmarkName = e.detail.value;

    // Display benchmark documentation if it exists.
    const docElement = $$('#benchmark_doc', this) as HTMLAnchorElement;
    if (benchmarkName && this._benchmarksToDocs[benchmarkName]) {
      docElement.hidden = false;
      docElement.href = this._benchmarksToDocs[benchmarkName];
    } else {
      docElement.hidden = true;
      docElement.href = '#';
    }

    // generic_trace_ct does not support parallel runs.
    if (benchmarkName === 'generic_trace_ct') {
      // runInParallel has ['True', 'False']. Select 'False'.
      this.runInParallel.selection = 1;
    }
  }

  _platformChanged(): void {
    const trueIndex = 0;
    const falseIndex = 1;
    const platform = this._platform();
    let offerGCETrue = true;
    let offerGCEFalse = true;
    let offerParallelTrue = true;
    if (platform === 'Android') {
      offerGCETrue = false;
      offerParallelTrue = false;
    } else if (platform === 'Windows') {
      offerGCEFalse = false;
    }
    // We default to use GCE for Linux, require if for Windows, and
    // disallow it for Android.
    (this.runOnGCE.children[trueIndex] as HTMLElement).hidden = !offerGCETrue;
    (this.runOnGCE.children[falseIndex] as HTMLElement).hidden = !offerGCEFalse;
    this.runOnGCE.selection = offerGCETrue ? trueIndex : falseIndex;

    // We default to run in parallel, except for Android which disallows it.
    (this.runInParallel.children[trueIndex] as HTMLElement).hidden = !offerParallelTrue;
    this.runInParallel.selection = offerParallelTrue ? trueIndex : falseIndex;

    this._updatePageSets();
  }

  _updatePageSets(): void {
    const platform = this._platform();
    const runInParallel = this._runInParallel();
    const unsupportedPS = (platform === 'Linux' && runInParallel)
      ? unsupportedPageSetsLinux
      : unsupportedPageSets;
    const pageSetDefault = (platform === 'Android')
      ? 'Mobile10k'
      : '10k';
    this.pageSets.hideKeys = unsupportedPS;
    this.pageSets.selected = pageSetDefault;
  }

  _platform(): string {
    return this._platforms[+this.platform.selection!][0];
  }

  _runInParallel(): boolean {
    return this.runInParallel.selection === 0;
  }

  _patchChanged(): void {
    this.description.value = combineClDescriptions(
      $('patch-sk', this).map((patch) => (patch as PatchSk).clDescription),
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
    if (missingLiveSitesWithCustomWebpages(
      this.pageSets.customPages,
      this.benchmarkArgs.value,
    )) {
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
    const params = {} as ChromiumAnalysisAddTaskVars;
    params.benchmark = this.benchmark.value;
    params.platform = this._platforms[+this.platform!.selection!][0];
    params.page_sets = this.pageSets.selected;
    params.run_on_gce = this.runOnGCE.selection === 0;
    params.match_stdout_txt = this.matchStdoutTxt.value;
    params.apk_gs_path = this.apkGsPath.value;
    params.chrome_build_gs_path = this.chromeBuildGsPath.value;
    params.telemetry_isolate_hash = this.telemetryIsolateHash.value;
    params.custom_webpages = this.pageSets.customPages;
    params.run_in_parallel = this.runInParallel.selection === 0;
    params.gn_args = this.gnArgs.value;
    params.benchmark_args = this.benchmarkArgs.value;
    params.browser_args = this.browserArgs.value;
    params.value_column_name = this.valueColumnName.value;
    params.desc = this.description.value;
    params.chromium_patch = this.chromiumPatch.patch;
    params.skia_patch = this.skiaPatch.patch;
    params.v8_patch = this.v8Patch.patch;
    params.catapult_patch = this.catapultPatch.patch;
    params.chromium_hash = this.chromiumHash.value;
    params.repeat_after_days = this.repeatAfterDays.frequency;
    params.task_priority = this.taskPriority.priority;
    if (this.ccList.value) {
      params.cc_list = this.ccList.value.split(',');
    }
    if (this.groupName.value) {
      params.group_name = this.groupName.value;
    }

    fetch('/_/add_chromium_analysis_task', {
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

  _gotoRunsHistory(): void {
    window.location.href = '/chromium_analysis_runs/';
  }
}

define('chromium-analysis-sk', ChromiumAnalysisSk);
