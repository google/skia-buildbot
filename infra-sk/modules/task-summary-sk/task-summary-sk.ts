/**
 * @module infra-sk/modules/task-summary-sk
 * @description <h2><code>task-summary-sk</code></h2>
 *
 * <p>
 * This element displays a task summary.
 * </p>
 *
 */
import { html, render } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { upgradeProperty } from '../../../elements-sk/modules/upgradeProperty';

// TODO(borenet): Consider using go2ts to generate this and other types.
export interface TaskSummary {
  Analysis?: string;
  ErrorMessage?: string;
}

export class TaskSummarySk extends HTMLElement {
  private static template = (ele: TaskSummarySk) => html`
    <div class="table">
      <div class="tr">
        <div class="td">Analysis</div>
        <div class="td">${ele.data.Analysis}</div>
      </div>
      <div class="tr">
        <div class="td">Error Message</div>
        <div class="td pre">${ele.data.ErrorMessage}</div>
      </div>
    </div>
  `;

  private _data: Partial<TaskSummary> = {};

  connectedCallback(): void {
    upgradeProperty(this, 'data');
    this.render();
  }

  get data(): TaskSummary {
    return this._data as TaskSummary;
  }

  set data(val: TaskSummary) {
    this._data = val;
    this.render();
  }

  private render() {
    render(TaskSummarySk.template(this), this, { host: this });
  }
}

define('task-summary-sk', TaskSummarySk);
