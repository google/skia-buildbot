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

import { $$, DomReady } from 'common-sk/modules/dom';
import { fromObject } from 'common-sk/modules/query';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { define } from 'elements-sk/define';
import 'elements-sk/select-sk';
import { errorMessage } from 'elements-sk/errorMessage';
import { html } from 'lit-html';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import {
  getFormattedTimestamp, taskDescriptors, getTimestamp, getCtDbTimestamp,
} from '../ctfe_utils';

const template = (el) => html`
<confirm-dialog-sk id="confirm_dialog"></confirm-dialog-sk>

<table class="options panel">
  <tr>
    <td>Benchmark Name</td>
    <td>
      <suggest-input-sk
        .options=${el.benchmarks}
        accept-custom-value
        .label="Hit <enter> at end if entering custom benchmark"
      ></suggest-input-sk>
      <!-- <template is="dom-if" if="{{getBenchmarkDoc(selectedBenchmarkName)}}">
        <div><a href="{{getBenchmarkDoc(selectedBenchmarkName)}}">Documentation</a></div>
      </template> -->
    </td>
  </tr>
  <tr>
    <td>Target Platform</td>
    <td>
      <select-sk>
        ${el.platforms.map((p, i) => (html`<div ?selected=${i === 0}>${p}</div>`))}
      </select-sk><!-- TODO this changes depending on the benchmark-->
    </td>
  </tr>
  <tr>
    <td>PagestSets Type</td>
    <td>
      <select-sk>
        ${el.pageSets.map((p, i) => (html`<div ?selected=${i === 0}>${p}</div>`))}
      </select-sk><!-- TODO expanding tezt area for custom stuff-->
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
    this._pageSets = ['Top 10K (with desktop user-agent)', 'Top 10K (with mobile user-agent)', 'Top 1K (with desktop user-agent, for testing, hidden from Runs History by default)', 'Top 1K (with mobile user-agent, for testing, hidden from Runs History by default)', 'Volt 10K (with mobile user-agent)'];
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
