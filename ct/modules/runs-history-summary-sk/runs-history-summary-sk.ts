/**
 * @fileoverview The bulk of the Runs History page.
 */

import 'elements-sk/tabs-sk';
import 'elements-sk/styles/buttons';

import { DomReady } from 'common-sk/modules/dom';
import { fromObject } from 'common-sk/modules/query';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { define } from 'elements-sk/define';
import { errorMessage } from 'elements-sk/errorMessage';
import { html } from 'lit-html';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { getCtDbTimestamp } from '../ctfe_utils';

import {
  CompletedTask,
  CompletedTaskResponse,
} from '../json';

export class RunsHistorySummarySk extends ElementSk {
  private _tasks: CompletedTask[] = [];

  private _uniqueUsers: number = 0;

  private _period: number = 0;

  constructor() {
    super(RunsHistorySummarySk.template);
  }

  private static template = (el: RunsHistorySummarySk) => html`
<div>
  <h4>CT Runs Summary</h4>
    <tabs-sk
      @tab-selected-sk=${(e: CustomEvent) => el.period = [7, 30, 365, 0][e.detail.index]}>
      <button>Last Week</button>
      <button>Last Month</button>
      <button>Last Year</button>
      <button>All Time</button>
    </tabs-sk>
    <br/><br/>
    <span>
      ${el._tasks.length} runs by ${el._uniqueUsers} users
      ${(el.period > 0) ? `last ${el.period} days` : 'all time'}
    </span>
    <br/>
<table class="queue surface-themes-sk secondary-links runssummary" id=runssummary>
  <tr>
    <th>Type</th>
    <th>User</th>
    <th>Description</th>
    <th>Completed</th>
  </tr>
  ${el._tasks.map((task: CompletedTask) => RunsHistorySummarySk.taskRowTemplate(task))}
 </table>
</div>
`;

  private static taskRowTemplate = (task: CompletedTask) => html`
<tr>
  <td>${task.type}</td>
  <td>${task.username}</td>
  <td>${task.description}</td>
  <td class="nowrap">${task.ts_completed}</td>
</tr>
`;

  connectedCallback(): void {
    super.connectedCallback();
    // We wait for everything to load so scaffolding event handlers are
    // attached.
    DomReady.then(() => {
      this.period = 7; // kicks off the reload.
    });
  }

  /**
   * @prop {Number} period - Number of days to look back for tasks.
   */
  get period(): number {
    return this._period;
  }

  set period(val: number) {
    this._period = val;
    this._reload();
  }

  _reload(): void {
    this.dispatchEvent(new CustomEvent('begin-task', { bubbles: true }));
    let completedAfter;
    if (this._period > 0) {
      const d = new Date();
      d.setDate(d.getDate() - this._period);
      completedAfter = getCtDbTimestamp(d);
    } else {
      completedAfter = getCtDbTimestamp(new Date(0));
    }

    const queryParams = {
      completed_after: completedAfter,
      exclude_ctadmin_tasks: true,
    };
    fetch(`/_/completed_tasks?${fromObject(queryParams)}`, { method: 'POST' })
      .then(jsonOrThrow)
      .then((json: CompletedTaskResponse) => {
        this._tasks = json.completed_tasks!;
        this._uniqueUsers = json.unique_users;
      })
      .catch((e) => {
        errorMessage(e);
      })
      .finally(() => {
        this._render();
        this.dispatchEvent(new CustomEvent('end-task', { bubbles: true }));
      });
  }
}

define('runs-history-summary-sk', RunsHistorySummarySk);
