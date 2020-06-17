/**
 * @fileoverview The bulk of the Performance page of CT.
 */

import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/icon/cancel-icon-sk';
import 'elements-sk/icon/check-circle-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/toast-sk';
import '../../../infra-sk/modules/confirm-dialog-sk';
import '../suggest-input-sk';
import '../input-sk';
import '../patch-sk';
import '../pageset-selector-sk';
import '../task-repeater-sk';
import '../task-priority-sk';

import { $$, DomReady, $ } from 'common-sk/modules/dom';
import { fromObject } from 'common-sk/modules/query';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { define } from 'elements-sk/define';
import 'elements-sk/select-sk';
import { errorMessage } from 'elements-sk/errorMessage';
import { html } from 'lit-html';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import {
  combineClDescriptions, missingLiveSitesWithCustomWebpages,
} from '../ctfe_utils';

// Chromium perf doesn't support !M and 100K pagesets.
const invalidPageSetStrings = ['All', '100k'];

const template = (el) => html`
<confirm-dialog-sk id=confirm_dialog></confirm-dialog-sk>

<table class="options panel">
  <tr>
    <td>Benchmark Name</td>
    <td>
      <suggest-input-sk
        id=benchmark_name
        .options=${el.benchmarks}
        .label=${'Hit <enter> at end if entering custom benchmark'}
        accept-custom-value
      ></suggest-input-sk>
      <!-- <template is="dom-if" if="{{getBenchmarkDoc(selectedBenchmarkName)}}">
        <div><a href="{{getBenchmarkDoc(selectedBenchmarkName)}}">Documentation</a></div>
      </template> -->
    </td>
  </tr>
  <tr>
    <td>Target Platform</td>
    <td>
      <select-sk @selection-changed=${el._platformChanged}>
        ${el.platforms.map((p, i) => (html`<div ?selected=${i === 1}>${p}</div>`))}
      </select-sk><!-- TODO this changes depending on the benchmark-->
    </td>
  </tr>
  <tr>
    <td>PagestSets Type</td>
    <td>
      <pageset-selector-sk id=pageset_selector .hideIfKeyContains=${invalidPageSetStrings}></pageset-selector-sk>
    </td>
  </tr>
  <tr>
    <td>
      Run in Parallel<br/>
      Read about the trade-offs <a href="https://docs.google.com/document/d/1GhqosQcwsy6F-eBAmFn_ITDF7_Iv_rY9FhCKwAnk9qQ/edit?pli=1#heading=h.xz46aihphb8z">here</a>
    </td>
    <td>
      <select-sk>
        <div>True</div>
        <div selected>False</div>
      </select-sk>
    </td>
  </tr>
  <tr>
    <td>Benchmark Arguments</td>
    <td>
      <input-sk value="--output-format=csv --pageset-repeat=1 --skip-typ-expectations-tags-validation --legacy-json-trace-format" id=benchmark_args class="long-field"></input-sk>
      <span class="smaller-font"><b>Note:</b> Change the --pageset-repeat value if you would like lower/higher repeats of each web page. 1 is the default.</span><br/>
      <span class="smaller-font"><b>Note:</b> Use --run-benchmark-timeout=[secs] to specify the timeout of the run_benchmark script. 300 is the default.</span><br/>
      <span class="smaller-font"><b>Note:</b> Use --max-pages-per-bot=[num] to specify the number of pages to run per bot. 100 is the default.</span>
    </td>
  </tr>
  <tr>
    <td>Browser Arguments (nopatch run)</td>
    <td>
      <input-sk value="" id=browser_args_nopatch class="long-field"></input-sk>
    </td>
  </tr>
  <tr>
    <td>Browser Arguments (withpatch run)</td>
    <td>
      <input-sk value="" id=browser_args_withpatch class="long-field"></input-sk>
    </td>
  </tr>
  <tr>
    <td>Field Value Column Name</td>
    <td>
      <input-sk value="avg" id=value_column_name class="medium-field"></input-sk>
      <span class="smaller-font">Which column's entries to use as field values.</span>
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
    <td>
      Chromium Git metrics patch (optional)<br/>
      Applied to Chromium ToT<br/>
      or to the hash specified below.<br/>
      Used to create the base build (See <a href="http://skbug.com/9029">skbug/9029</a>)
    </td>
    <td>
      <patch-sk id=chromium_patch_base_build
                patchType=chromium
                @cl-description-changed=${el._patchChanged}>
      </patch-sk>
    </td>
  </tr>

  <tr>
    <td>Chromium hash to sync to (optional)<br/></td>
    <td>
      <input-sk value="" id=chromium_hash class="long-field"></input-sk>
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
      <task-priority-sk></task-priority-sk>
    </td>
  </tr>
  <tr>
    <td>
      Notifications CC list (optional)<br/>
      Email will be sent by ct@skia.org
    </td>
    <td>
      <input-sk value="" id=cc_list label="email1,email2,email3" class="long-field"></input-sk>
    </td>
  </tr>
  <tr>
    <td>
      Group name (optional)<br/>
      Will be used to track runs
    </td>
    <td>
      <input-sk value="" id=group_name class="long-field"></input-sk>
    </td>
  </tr>
  <tr>
    <td>Description</td>
    <td>
      <input-sk value="" id=description label="Description is required" class="long-field"></input-sk>
    </td>
  </tr>
  <tr>
    <td colspan="2" class="center">
      <div class="triggering-spinner">
        <spinner-sk active=${el._submitting} alt="Trigger task"></spinner-sk>
      </div>
      <button ?disabled=${el._submitting} @click=${el._validateTask}>Queue Task</button>
    </td>
  </tr>
  <tr>
    <td colspan="2" class="center">
      <button id="view_history">View runs history</button>
    </td>
  </tr>


</table>
`;

define('chromium-perf-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._running = false;

    this._benchmarks = ['ad_tagging.cluster_telemetry', 'generic_trace_ct', 'leak_detection.cluster_telemetry', 'loading.cluster_telemetry', 'memory.cluster_telemetry', 'rasterize_and_record_micro_ct', 'rendering.cluster_telemetry', 'repaint_ct', 'usecounter_ct', 'v8.loading.cluster_telemetry', 'v8.loading_runtime_stats.cluster_telemetry'];
    this._platforms = ['Android (Pixel2 devices)', 'Linux (Ubuntu18.04 machines)', 'Windows (2016 DataCenter Server cloud instances)'];
    this._pageSets = [];//['Top 10K (with desktop user-agent)', 'Top 10K (with mobile user-agent)', 'Top 1K (with desktop user-agent, for testing, hidden from Runs History by default)', 'Top 1K (with mobile user-agent, for testing, hidden from Runs History by default)', 'Volt 10K (with mobile user-agent)'];
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
      /* this.loadTaskQueue().then(() => {
        this._render();
        this._running = false;
        this.dispatchEvent(new CustomEvent('end-task', { bubbles: true }));
      }); */
    });

    this._render();
    this._pagesetSelector = $$('pageset-selector-sk', this);
  }

  /**
   * @prop {Array<string>} benchmarks - agsd
   */
  get benchmarks() {
    return this._benchmarks;
  }

  set benchmarks(v) {
    this._benchmarks = v;
  }

  /**
   * @prop {Array<string>}  platforms - agsd
   */
  get platforms() {
    return this._platforms;
  }

  set platforms(v) {
    this._platforms = v;
  }

  /**
   * @prop {Array<string>}  pageSets - agsd
   */
  get pageSets() {
    return this._pageSets;
  }

  set pageSets(v) {
    this._pageSets = v;
  }

  _platformChanged(e) {
    if (e.detail.selection === 0) {
      this._pagesetSelector.selected = 'Mobile10k';
    } else {
      this._pagesetSelector.selected = '10k';
    }
  }

  _patchChanged() {
    $$('#description', this).value = combineClDescriptions(
      $('patch-sk', this).map((patch) => patch.clDescription)
    );
  }

  _validateTask() {
    if (!$('patch-sk', this).every((patch) => {console.log('validating a patch'); return patch.validate(); })) {
      return;
    }
    if (!$$('#description', this).value) {
      errorMessage('Please specify a description');
      $$('#description', this).focus();
      return;
    }
    if (!$$('#benchmark_name', this).value) {
      errorMessage('Please specify a benchmark');
      $$('#benchmark_name', this).focus();
      return;
    }
    if (missingLiveSitesWithCustomWebpages(
            $$('#pageset_selector', this).customPages, $$('#benchmark_args', this).value) {
      this.$.benchmark_args.focus();
      return;
    }
    if (ctfe.moreThanThreeActiveTasks($$$('drawer-sk').sizeOfUserQueue)) {
      return;
    }
    this.$.confirm_dialog.open('Proceed with queueing task?')
      .then(this.queueTask.bind(this))
      .catch(function() {
        sk.errorMessage('Did not queue');
      })
  }
/*
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
  } */
});
