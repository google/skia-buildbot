/**
 * @module module/cluster-lastn-page-sk
 * @description <h2><code>cluster-lastn-page-sk</code></h2>
 *
 *  Allows trying out an alert by clustering over a range of commits.
 */
import dialogPolyfill from 'dialog-polyfill';
import { define } from 'elements-sk/define';
import { errorMessage } from 'elements-sk/errorMessage';
import { fromObject } from 'common-sk/modules/query';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { stateReflector } from 'common-sk/modules/stateReflector';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import 'elements-sk/spinner-sk';
import 'elements-sk/styles/buttons';

import '../cluster-summary2-sk';
import '../commit-detail-sk';
import '../triage-status-sk';
import '../alert-config-sk';
import '../domain-picker-sk';

function _stepUpAt(dir) {
  return dir === 'UP' || dir === 'BOTH';
}

function _stepDownAt(dir) {
  return dir === 'DOWN' || dir === 'BOTH';
}

function _notBoth(dir) {
  return dir !== 'BOTH';
}

const _tableHeader = (ele) => {
  const ret = [html`<th></th>`];
  if (_stepDownAt(ele._state.direction)) {
    ret.push(html`<th>Low</th>`);
  }
  if (_stepUpAt(ele._state.direction)) {
    ret.push(html`<th>High</th>`);
  }
  if (_notBoth(ele._state.direction)) {
    ret.push(html`<th></th>`);
  }
  return ret;
};

function _full_summary(frame, summary) {
  return {
    frame,
    summary,
  };
}

const _low = (ele, reg) => {
  if (!_stepDownAt(ele._state.direction)) {
    return html``;
  }
  if (reg.regression.low) {
    return html`
      <td class=cluster>
        <triage-status-sk
          .alert=${ele._state}
          cluster_type=low
          .full_summary=${_full_summary(reg.regression.frame, reg.regression.low)}
          .triage=${reg.regression.low_status}>
        </triage-status-sk>
      </td>
    `;
  }
  return html`
      <td class=cluster>
      </td>
    `;
};

const _high = (ele, reg) => {
  if (!_stepUpAt(ele._state.direction)) {
    return html``;
  }
  if (reg.regression.high) {
    return html`
      <td class=cluster>
        <triage-status-sk
          .alert=${ele._state}
          cluster_type=high
          .full_summary=${_full_summary(reg.regression.frame, reg.regression.high)}
          .triage=${reg.regression.high_status}>
        </triage-status-sk>
      </td>
    `;
  }
  return html`
      <td class=cluster>
      </td>
    `;
};

const _filler = (ele) => {
  if (_notBoth(ele._state.direction)) {
    return html`<td></td>`;
  }
  return html``;
};

const _tableRows = (ele) => ele._regressions.map((reg) => html`
  <tr>
    <td class=fixed>
      <commit-detail-sk .cid=${reg.cid}></commit-detail-sk>
    </td>

    ${_low(ele, reg)}
    ${_high(ele, reg)}
    ${_filler(ele)}
  </tr>
  `);

const _configTitle = (ele) => html`Algo: ${ele._state.algo} - Radius: ${ele._state.radius} - Sparse: ${ele._state.sparse} - Threshold: ${ele._state.interesting}`;

const _table = (ele) => {
  if (ele._requestId && !ele._regressions.length) {
    return html`No regressions found yet.`;
  }
  return html`
    <table @start-triage=${ele._triage_start}>
      <tr>
        <th>Commit</th>
        <th colspan=2>Regressions</th>
      </tr>
      <tr>
        ${_tableHeader(ele)}
      </tr>
      ${_tableRows(ele)}
    </table>`;
};

const template = (ele) => html`
  <dialog id=alert-config-dialog>
  <alert-config-sk
    .config=${ele._state}
    .paramset=${ele._paramset}
    .key_order=${window.sk.perf.key_order}
    ></alert-config-sk>
    <div class=buttons>
      <button @click=${ele._alertClose}>Cancel</button>
      <button @click=${ele._alertAccept}>Accept</button>
    </div>
  </dialog>
  <div class=controls>
    <label><h2>Alert Configuration</h2> <button @click=${ele._alertEdit}>${_configTitle(ele)}</button></label>
    <label><h2>Time Range</h2> <domain-picker-sk id=range .state=${ele._domain} force_request_type=dense></domain-picker-sk></label>
    <div class=running>
      <button class=action ?disabled=${!ele._state.query || !!ele._requestId} @click=${ele._run}>Run</button>
      <spinner-sk ?active=${!!ele._requestId}></spinner-sk>
      <pre class=messages>${ele._runningStatus}</pre>
    </div>
  </div>
  <hr>

  <dialog id=triage-cluster-dialog  @open-keys=${ele._openKeys}>
  <cluster-summary2-sk
    .full_summary=${ele._dialog_state.full_summary}
    .triage=${ele._dialog_state.triage}
    notriage
  ></cluster-summary2-sk>
    <div class=buttons>
      <button @click=${ele._triageClose}>Close</button>
    </div>
  </dialog>

  ${_table(ele)}
`;

define('cluster-lastn-page-sk', class extends ElementSk {
  constructor() {
    super(template);

    // The fetch in connectedCallback will fill this is with the defaults.
    this._state = {};

    // The range of commits over which we are clustering.
    this._domain = {
      end: Math.floor(Date.now() / 1000),
      num_commits: 200,
      request_type: 1,
    };

    // The regressions found when running.
    this._regressions = [];

    // The state of the cluster-summary2-sk dialog.
    this._dialog_state = {};

    // The paramsets for the alert config.
    this._paramset = {};

    // The id of the currently running request.
    this._requestId = null;

    // The text status of the currently running request.
    this._runningStatus = '';
  }

  connectedCallback() {
    super.connectedCallback();
    const init = fetch('/_/initpage/').then(jsonOrThrow).then((json) => {
      this._paramset = json.dataframe.paramset;
    });
    const alertNew = fetch('/_/alert/new').then(jsonOrThrow).then((json) => {
      this._state = json;
    });
    Promise.all([init, alertNew]).then(() => {
      this._render();
      this._alertDialog = this.querySelector('#alert-config-dialog');
      this._triageDialog = this.querySelector('#triage-cluster-dialog');
      dialogPolyfill.registerDialog(this._alertDialog);
      dialogPolyfill.registerDialog(this._triageDialog);
      this._alertConfig = this.querySelector('alert-config-sk');
      this._stateHasChanged = stateReflector(() => this._state, (state) => {
        this._state = state;
        this._render();
      });
    }).catch(errorMessage);
  }

  _alertEdit() {
    this._alertDialog.showModal();
  }

  _alertClose() {
    this._alertDialog.close();
  }

  _alertAccept() {
    this._alertDialog.close();
    this._state = this._alertConfig.config;
    this._stateHasChanged();
    this._render();
  }

  _triage_start(e) {
    this._dialog_state = e.detail;
    this._render();
    this._triageDialog.show();
  }

  _triageClose() {
    this._triageDialog.close();
  }

  _openKeys(e) {
    const query = {
      keys: e.detail.shortcut,
      begin: e.detail.begin,
      end: e.detail.end,
      xbaroffset: e.detail.xbar.offset,
    };
    window.open(`/e/?${fromObject(query)}`, '_blank');
  }

  _catch(msg) {
    this._requestId = null;
    this._runningStatus = '';
    this._render();
    if (msg) {
      errorMessage(msg, 10000);
    }
  }

  _run() {
    if (this._requestId) {
      errorMessage('There is a pending query already running.');
      return;
    }
    this._domain = this.querySelector('#range', this).state;
    const body = {
      domain: this._domain,
      config: this._state,
    };
    fetch('/_/dryrun/start', {
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
      },
    }).then(jsonOrThrow).then((json) => {
      this._requestId = json.id;
      this._render();
      this._checkDryRunStatus((regressions) => {
        this._regressions = regressions;
        this._render();
      });
    }).catch((msg) => this._catch(msg));
  }

  _checkDryRunStatus(cb) {
    fetch(`/_/dryrun/status/${this._requestId}`).then(jsonOrThrow).then((json) => {
      this._runningStatus = json.message;
      if (!json.finished) {
        window.setTimeout(() => this._checkDryRunStatus(cb), 300);
      } else {
        this._requestId = null;
      }
      // json.regressions will get filled in incrementally, so display them
      // as they arrive.
      cb(json.regressions);
    }).catch((msg) => this._catch(msg));
  }
});
