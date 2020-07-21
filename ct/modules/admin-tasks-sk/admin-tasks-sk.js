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
import '../../../infra-sk/modules/confirm-dialog-sk';
import '../chromium-build-selector-sk';
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

import {
  moreThanThreeActiveTasksChecker,
} from '../ctfe_utils';

const template = (el) => html`
<confirm-dialog-sk id=confirm_dialog></confirm-dialog-sk>

<div>
  <tabs-sk @tab-selected-sk=${el._setActiveTab}>
    <button class=selected>Recreate Pagesets</button>
    <button>Recreate Webpage Archives</button>
  </tabs-sk>
  <tabs-panel-sk>
    <div id=pagesets>${tabTemplate(el, false)}</div>
    <div id=archives>${tabTemplate(el, true)}</div>
  </tabs-panel-sk>
</div>
`;

const tabTemplate = (el, showChromeBuild) => html`
<table class=options>
  <tr>
    <td>PageSets Type</td>
    <td>
      <pageset-selector-sk id=pageset_selector disable-custom-webpages>
      </pageset-selector-sk>
    </td>
  </tr>
   ${showChromeBuild ? html`
  <tr>
    <td>Chromium Build</td>
    <td>
      <chromium-build-selector-sk id=chromium_build></chromium-build-selector-sk>
    </td>
  </tr>` : html``}
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

define('admin-tasks-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._triggeringTask = false;
    this._moreThanThreeActiveTasks = moreThanThreeActiveTasksChecker();
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    this._activeTab = $('tabs-panel-sk div')[0];
  }

  _setActiveTab(e) {
    // For template simplicity we have some same-IDed elements, we use the
    // active tab as the parent in selectors.
    this._activeTab = $('tabs-panel-sk>div', this)[e.detail.index];
  }

  _validateTask() {
    if (!$$('#pageset_selector', this._activeTab).selected) {
      errorMessage('Please select a page set type');
      $$('#pageset_selector', this).focus();
      return;
    }
    if (this._activeTab.id === 'archives'
    && !$$('#chromium_build', this._activeTab).build) {
      errorMessage('Please select a Chromium build');
      $$('#chromium_build', this).focus();
      return;
    }
    if (this._moreThanThreeActiveTasks()) {
      return;
    }
    $$('#confirm_dialog', this).open('Proceed with queueing task?')
      .then(() => this._queueTask())
      .catch(() => {
        errorMessage('Unable to queue task');
      });
  }

  _queueTask() {
    this._triggeringTask = true;
    const params = {};
    params.page_sets = $$('#pageset_selector', this).selected;
    params.repeat_after_days = $$('#repeat_after_days', this).frequency;

    let url = '/_/add_recreate_page_sets_task';
    if (this._activeTab.id === 'archives') {
      params.chromium_build = $$('#chromium_build', this).build;
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

  _gotoRunsHistory() {
    if (this._activeTab.id === 'archives') {
      window.location.href = '/recreate_webpage_archives_runs/';
    } else {
      window.location.href = '/recreate_page_sets_runs/';
    }
  }
});
