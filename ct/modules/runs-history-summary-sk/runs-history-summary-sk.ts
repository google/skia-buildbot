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

const template = (el) => html`
<div>
  <h4>CT Runs Summary</h4>
    <tabs-sk
      @tab-selected-sk=${(e) => el.period = [7, 30, 365, 0][e.detail.index]}>
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
  <tr class=primary-variant-container-themes-sk>
    <th>Type</th>
    <th>User</th>
    <th>Description</th>
    <th>Completed</th>
  </tr>
  ${el._tasks.map((task) => taskRowTemplate(task))}
 </table>
</div>
`;

const taskRowTemplate = (task) => html`
<tr>
  <td>${task.Type}</td>
  <td>${task.Username}</td>
  <td>${task.Description}</td>
  <td class="nowrap">${task.TsCompleted}</td>
</tr>
`;

define('runs-history-summary-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._tasks = [];
    this._uniqueUsers = 0;
  }

  connectedCallback() {
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
  get period() {
    return this._period;
  }

  set period(val) {
    this._period = val;
    this._reload();
  }

  _reload() {
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
      .then((json) => {
        this._tasks = json.CompletedTasks;
        this._uniqueUsers = json.UniqueUsers;
      })
      .catch((e) => {
        errorMessage(e);
      })
      .finally(() => {
        this._render();
        this.dispatchEvent(new CustomEvent('end-task', { bubbles: true }));
      });
  }
});
