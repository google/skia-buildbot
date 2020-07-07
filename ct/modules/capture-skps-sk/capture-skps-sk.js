/**
 * @fileoverview The bulk of the Capture SKPs page of CT.
 */

import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/icon/cancel-icon-sk';
import 'elements-sk/icon/check-circle-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/toast-sk';
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

// Capture SKPs doesn't support 1M pagesets.
const unsupportedPageSetStrings = ['All'];

const template = (el) => html`
<confirm-dialog-sk id=confirm_dialog></confirm-dialog-sk>

<table class=options>
  <tr>
    <td>PageSets Type</td>
    <td>
      <pageset-selector-sk id=pageset_selector
        disable-custom-webpages
        .hideIfKeyContains=${unsupportedPageSetStrings}>
      </pageset-selector-sk>
    </td>
  </tr>
  <tr>
    <td>Chromium Build</td>
    <td>
      <chromium-build-selector-sk id=chromium_build></chromium-build-selector-sk>
    </td>
  </tr>
  <tr>
    <td>Repeat this task</td>
    <td>
      <task-repeater-sk id=repeat_after_days></task-repeater-sk>
    </td>
  </tr>
  <tr>
    <td>Description</td>
    <td>
      <input-sk value="" id=description
        label="Description is required. Please include SKP version."
        class=long-field>
      </input-sk>
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

define('capture-skps-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._triggeringTask = false;
    this._moreThanThreeActiveTasks = moreThanThreeActiveTasksChecker();
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

  _validateTask() {
    if (!$$('#pageset_selector', this).selected) {
      errorMessage('Please select a page set type');
      $$('#pageset_selector', this).focus();
      return;
    }
    if (!$$('#chromium_build', this).build) {
      errorMessage('Please select a Chromium build');
      $$('#chromium_build', this).focus();
      return;
    }
    if (!$$('#description', this).value) {
      errorMessage('Please specify a description');
      $$('#description', this).focus();
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
    params.chromium_build = $$('#chromium_build', this).build;
    params.repeat_after_days = $$('#repeat_after_days', this).frequency;
    params.desc = $$('#description', this).value;

    fetch('/_/add_capture_skps_task', {
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
    window.location.href = '/capture_skp_runs/';
  }
});
