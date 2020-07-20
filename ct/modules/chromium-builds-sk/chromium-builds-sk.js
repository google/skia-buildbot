/**
 * @fileoverview The bulk of the Chromium Builds page of CT.
 */

import 'elements-sk/icon/delete-icon-sk';
import 'elements-sk/icon/cancel-icon-sk';
import 'elements-sk/icon/check-circle-icon-sk';
import 'elements-sk/icon/help-icon-sk';
import 'elements-sk/spinner-sk';
import 'elements-sk/toast-sk';
import '../../../infra-sk/modules/confirm-dialog-sk';
import '../chromium-build-selector-sk';
import '../suggest-input-sk';
import '../input-sk';
import '../pageset-selector-sk';
import '../task-repeater-sk';

import { $$ } from 'common-sk/modules/dom';
import { fromObject } from 'common-sk/modules/query';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
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

<table class=options>
  <tr>
    <td>Chromium Commit Hash</td>
    <td>
      <input-sk id=chromium_rev value=LKGR
        @input=${el._chromiumRevChanged} class=hash-field>
      </input-sk>
    </td>
    <td>
      <div class="rev-detail-container">
        <div class="loading-rev-spinner">
          <spinner-sk id=chromium_spinner alt="Loading Chromium commit details"></spinner-sk>
        </div>
        <div class="rev-detail">${el._formatRevData(el._chromiumRevData)}</div>
      </div>
    </td>
  </tr>
  <tr>
    <td>Skia Commit Hash</td>
    <td>
      <input-sk id=skia_rev value=LKGR  @input=${el._skiaRevChanged} class=hash-field></input-sk>
    </td>
    <td>
      <div class="rev-detail-container">
        <div class="loading-rev-spinner">
          <spinner-sk id=skia_spinner alt="Loading Skia commit details"></spinner-sk>
        </div>
        <div class="rev-detail">${el._formatRevData(el._skiaRevData)}</div>
      </div>
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

define('chromium-builds-sk', class extends ElementSk {
  constructor() {
    super(template);
    this._triggeringTask = false;
    this._moreThanThreeActiveTasks = moreThanThreeActiveTasksChecker();
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
    // Load LKGR data.
    this._chromiumRevChanged();
    this._skiaRevChanged();
  }


  _chromiumRevChanged() {
    const spinner = $$('#chromium_spinner', this);
    const newValue = $$('#chromium_rev', this).value;
    this._chromiumRev = newValue;
    if (!newValue) {
      this._chromiumRevData = null;
      spinner.active = false;
      return;
    }
    spinner.active = true;
    const params = { rev: newValue };
    const url = `/_/chromium_rev_data?${fromObject(params)}`;

    fetch(url, { method: 'POST' })
      .then(jsonOrThrow)
      .then((json) => {
        if (this._chromiumRev === newValue) {
          if (json.commit) {
            this._chromiumRevData = json;
          } else {
            this._chromiumRevData = null;
          }
        }
      })
      .catch((err) => {
        if (this._chromiumRev === newValue) {
          this._chromiumRevData = { error: err };
        }
      })
      .finally(() => {
        if (this._chromiumRev === newValue) {
          spinner.active = false;
        }
        this._render();
      });
  }

  _skiaRevChanged() {
    const spinner = $$('#skia_spinner', this);
    const newValue = $$('#skia_rev', this).value;
    this._skiaRev = newValue;
    if (!newValue) {
      this._skiaRevData = null;
      spinner.active = false;
      return;
    }
    spinner.active = true;
    const params = { rev: newValue };
    const url = `/_/skia_rev_data?${fromObject(params)}`;

    fetch(url, { method: 'POST' })
      .then(jsonOrThrow)
      .then((json) => {
        if (this._skiaRev === newValue) {
          if (json.commit) {
            this._skiaRevData = json;
          } else {
            this._skiaRevData = null;
          }
        }
      })
      .catch((err) => {
        if (this._skiaRev === newValue) {
          this._skiaRevData = { error: err };
        }
      })
      .finally(() => {
        if (this._skiaRev === newValue) {
          spinner.active = false;
        }
        this._render();
      });
  }

  _formatRevData(revData) {
    if (revData) {
      if (!revData.error) {
        return `${revData.commit} by ${revData.author.name} submitted ${
          revData.committer.time}`;
      }
      return revData.error;
    }
    return '';
  }

  _validateTask() {
    if (!this._chromiumRevData || !this._chromiumRevData.commit) {
      errorMessage('Please enter a valid Chromium commit hash.');
      $$('#chromium_rev', this).focus();
      return;
    }
    if (!this._skiaRevData || !this._skiaRevData.commit) {
      errorMessage('Please enter a valid Skia commit hash.');
      $$('#skia_rev', this).focus();
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
    params.chromium_rev = this._chromiumRevData.commit;
    params.chromium_rev_ts = this._chromiumRevData.committer.time;
    params.skia_rev = this._skiaRevData.commit;
    params.repeat_after_days = $$('#repeat_after_days', this).frequency;

    fetch('/_/add_chromium_build_task', {
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
    window.location.href = '/chromium_builds_runs/';
  }
});
