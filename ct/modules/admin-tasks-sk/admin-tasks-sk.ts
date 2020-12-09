/**
 * @fileoverview The bulk of the Admin Tasks page of CT.
 */

import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/icon/cancel-icon-sk';
import 'elements-sk/icon/check-circle-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/toast-sk';
import 'elements-sk/tabs-sk';
import 'elements-sk/tabs-panel-sk';
import '../suggest-input-sk';
import '../input-sk';
import '../pageset-selector-sk';
import '../task-repeater-sk';

import { $$, $ } from 'common-sk/modules/dom';
import { define } from 'elements-sk/define';
import 'elements-sk/select-sk';
import { errorMessage } from 'elements-sk/errorMessage';
import { html } from 'lit-html';

import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import { PagesetSelectorSk } from '../pageset-selector-sk/pageset-selector-sk';
import { TaskRepeaterSk } from '../task-repeater-sk/task-repeater-sk';
import { AdminAddTaskVars } from '../json';
import {
  moreThanThreeActiveTasksChecker,
} from '../ctfe_utils';

export class AdminTasksSk extends ElementSk {
  _activeTab: Element | null = null;

  private _triggeringTask = false;

  private _moreThanThreeActiveTasks = moreThanThreeActiveTasksChecker();

  constructor() {
    super(AdminTasksSk.template);
  }

  private static template = (el: AdminTasksSk) => html`

<div>
  <tabs-sk @tab-selected-sk=${el._setActiveTab}>
    <button class=selected>Recreate Pagesets</button>
    <button>Recreate Webpage Archives</button>
  </tabs-sk>
  <tabs-panel-sk>
    <div id=pagesets>${AdminTasksSk.tabTemplate(el)}</div>
    <div id=archives>${AdminTasksSk.tabTemplate(el)}</div>
  </tabs-panel-sk>
</div>
`;

  private static tabTemplate = (el: AdminTasksSk) => html`
<table class=options>
  <tr>
    <td>PageSets Type</td>
    <td>
      <pageset-selector-sk id=pageset_selector disable-custom-webpages>
      </pageset-selector-sk>
    </td>
  </tr>
  <tr>
    <td>Repeat this task</td>
    <td>
      <task-repeater-sk id=repeat_after_days></task-repeater-sk>
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
    this._activeTab = $('tabs-panel-sk div')[0];
  }

  _setActiveTab(e: CustomEvent): void {
  // For template simplicity we have some same-IDed elements, we use the
  // active tab as the parent in selectors.
    this._activeTab = $('tabs-panel-sk>div', this)[e.detail.index];
  }

  _validateTask(): void {
    const pagesetSelector = $$('#pageset_selector', this._activeTab!) as PagesetSelectorSk;
    if (!pagesetSelector.selected) {
      errorMessage('Please select a page set type');
      pagesetSelector.focus();
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
    const params = {} as AdminAddTaskVars;
    params.page_sets = ($$('#pageset_selector', this._activeTab!) as PagesetSelectorSk).selected;
    params.repeat_after_days = ($$('#repeat_after_days', this._activeTab!) as TaskRepeaterSk).frequency;

    let url = '/_/add_recreate_page_sets_task';
    if (this._activeTab!.id === 'archives') {
      url = '/_/add_recreate_webpage_archives_task';
    }

    fetch(url, {
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
    if (this._activeTab!.id === 'archives') {
      window.location.href = '/recreate_webpage_archives_runs/';
    } else {
      window.location.href = '/recreate_page_sets_runs/';
    }
  }
}

define('admin-tasks-sk', AdminTasksSk);
