/**
 * @fileoverview The bulk of the Metrics Analysis page of CT.
 */

import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/icon/cancel-icon-sk';
import 'elements-sk/icon/check-circle-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/toast-sk';
import '../suggest-input-sk';
import '../input-sk';
import '../patch-sk';
import '../task-repeater-sk';
import '../task-priority-sk';

import { $$, $ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import 'elements-sk/select-sk';
import { errorMessage } from 'elements-sk/errorMessage';
import { html } from 'lit-html';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import { InputSk } from '../input-sk/input-sk';
import { PatchSk } from '../patch-sk/patch-sk';
import { TaskPrioritySk } from '../task-priority-sk/task-priority-sk';
import { TaskRepeaterSk } from '../task-repeater-sk/task-repeater-sk';
import {
  combineClDescriptions,
  moreThanThreeActiveTasksChecker,
} from '../ctfe_utils';
import { MetricsAnalysisAddTaskVars } from '../json';

interface ExpandableTextArea extends InputSk {
  open: boolean;
}

export class MetricsAnalysisSk extends ElementSk {
  private _triggeringTask: boolean = false;

  private _moreThanThreeActiveTasks = moreThanThreeActiveTasksChecker();

  // Variables that represent UI elements.
  private metricName!: InputSk;

  private analysisTaskId!: InputSk;

  private customTraces!: ExpandableTextArea;

  private benchmarkArgs!: InputSk;

  private valueColumnName!: InputSk;

  private description!: InputSk;

  private chromiumPatch!: PatchSk;

  private catapultPatch!: PatchSk;

  private repeatAfterDays!: TaskRepeaterSk;

  private taskPriority!: TaskPrioritySk;

  private ccList!: InputSk;

  constructor() {
    super(MetricsAnalysisSk.template);
  }

  private static template = (el: MetricsAnalysisSk) => html`

<table class=options>
  <tr>
    <td>Metric Name</td>
    <td>
      <input-sk
        id=metric_name
        label="The metric to parse the provided traces with. Eg: loadingMetric"
        class=medium-field
      ></input-sk>
    </td>
  </tr>
  <tr>
    <td>Source of Traces</td>
    <td>
      <input-sk
        id=analysis_task_id
        label="Analysis Task Id"
        class=medium-field
      ></input-sk>
      <expandable-textarea-sk
        id=custom_traces
        minRows=5
        displaytext="Specify custom list of traces"
        placeholder="Eg: trace1,trace2,trace3"
        @click=${el._toggleAnalysisTaskId}>
      </expandable-textarea-sk>
    </td>
  </tr>
  <tr>
    <td>Benchmark Arguments</td>
    <td>
      <input-sk value="--output-format=csv" id=benchmark_args class=long-field></input-sk>
      <span class=smaller-font>These will be the arguments to the analysis_metrics_ct benchmark.</span><br/>
      <span class=smaller-font><b>Note:</b> Use --run-benchmark-timeout=[secs] to specify the timeout of the run_benchmark script. 300 is the default.</span><br/>
      <span class=smaller-font><b>Note:</b> Use --max-pages-per-bot=[num] to specify the number of pages to run per bot. 50 is the default.</span>
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
      <button id=submit ?disabled=${el._triggeringTask} @click=${el._validateTask}>Queue Task
      </button>
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

    this.metricName = $$<InputSk>('#metric_name', this)!;
    this.analysisTaskId = $$<InputSk>('#analysis_task_id', this)!;
    this.customTraces = $$<ExpandableTextArea>('#custom_traces', this)!;
    this.benchmarkArgs = $$<InputSk>('#benchmark_args', this)!;
    this.valueColumnName = $$<InputSk>('#value_column_name', this)!;
    this.description = $$<InputSk>('#description', this)!;
    this.chromiumPatch = $$<PatchSk>('#chromium_patch', this)!;
    this.catapultPatch = $$<PatchSk>('#catapult_patch', this)!;
    this.repeatAfterDays = $$<TaskRepeaterSk>('#repeat_after_days', this)!;
    this.taskPriority = $$<TaskPrioritySk>('#task_priority', this)!;
    this.ccList = $$<InputSk>('#cc_list', this)!;
  }

  _toggleAnalysisTaskId(): void {
    if (this.customTraces.open === this.analysisTaskId.hidden) {
      // This click wasn't toggling the expandable textarea.
      return;
    }
    this.analysisTaskId.hidden = this.customTraces.open;
    // We assume if someone opens the custom traces window they will be using
    // custom traces, and if they close it, they won't be.
    if (this.customTraces.open) {
      this.customTraces.value = '';
    } else {
      this.customTraces.value = '';
    }
    this._render();
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
    if (!this.metricName.value) {
      errorMessage('Please specify a metric name');
      this.metricName.focus();
      return;
    }
    if (!this.analysisTaskId.value && !this.customTraces.value) {
      errorMessage('Please specify an analysis task id or custom traces');
      this.analysisTaskId.focus();
      return;
    }
    if (!this.description.value) {
      errorMessage('Please specify a description');
      this.description.focus();
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
    const params = {} as MetricsAnalysisAddTaskVars;
    params.metric_name = this.metricName.value;
    params.analysis_task_id = this.analysisTaskId.value;
    params.custom_traces = this.customTraces.value;
    params.benchmark_args = this.benchmarkArgs.value;
    params.value_column_name = this.valueColumnName.value;
    params.desc = this.description.value;
    params.chromium_patch = this.chromiumPatch.patch;
    params.catapult_patch = this.catapultPatch.patch;
    params.repeat_after_days = this.repeatAfterDays.frequency;
    params.task_priority = this.taskPriority.priority;
    if (this.ccList.value) {
      params.cc_list = this.ccList.value.split(',');
    }

    fetch('/_/add_metrics_analysis_task', {
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
    window.location.href = '/metrics_analysis_runs/';
  }
}

define('metrics-analysis-sk', MetricsAnalysisSk);