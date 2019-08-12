/**
 * @module module/cluster-lastn-page-sk
 * @description <h2><code>cluster-lastn-page-sk</code></h2>
 *
 *  Allows trying out an alert by clustering over a range of commits.
 */
import { html, render } from 'lit-html'
import { ElementSk } from '../../../infra-sk/modules/ElementSk'

function _stepUpAt(dir) {
  return dir === 'UP' || dir === 'BOTH';
}

function _stepDownAt(dir) {
  return dir === 'DOWN' || dir === 'BOTH';
}

function _notBoth(dir) {
  return dir != 'BOTH';
}

const _tableHeader = (ele) => {
  ret = [html`<th></th>`];
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
}

const _low = (ele, reg) => {
  if (!_stepDownAt(ele)) {
    return html``;
  }
  if (reg.regression.low) {
    return html`
      <td class=cluster>
        <triage-status-sk
          .alert=${ele._state}i
          cluster_type=low
          .full_summary=${ele._full_summary(reg.regression.frame, reg.regression.low)}
          .triage=${reg.regression.low_status}>
        </triage-status-sk>
      </td>
    `;
  } else {
    return html`
      <td class=cluster>
      </td>
    `;
  }
}

const _high = (ele, reg) => {
  if (!_stepUpAt(ele)) {
    return html``;
  }
  if (reg.regression.high) {
    return html`
      <td class=cluster>
        <triage-status-sk
          .alert=${ele._state}i
          cluster_type=high
          .full_summary=${ele._full_summary(reg.regression.frame, reg.regression.high)}
          .triage=${reg.regression.high_status}>
        </triage-status-sk>
      </td>
    `;
  } else {
    return html`
      <td class=cluster>
      </td>
    `;
  }
}

const _filler = (ele) => {
  if (_notBoth(ele)) {
    return html`<td></td>`;
  }
  return html``;
}

const _tableRows = (ele) => ele._regressions.map((reg, rowIndex) => html`
  <tr>
    <td class=fixed>
      <commit-detail-sk .cid=${reg.cid}></commit-detail-sk>
    </td>

    ${_low(ele, reg)}
    ${_high(ele, reg)}
    ${_filler(ele)}
  </tr>
  `);

const template = (ele) => html`
  <dialog>
    <alert-config-sk id=alertconfig .config=${ele._state}></alert-config-sk>
    <div class=buttons>
      <button @click=${ele._close}>Cancel</button>
      <button>Accept</button>
    </div>
  </dialog>
  <div class=controls>
    <label>Alert Configuration: <button @click=${ele._editAlert}>${ele._configTitle()}</button></label>
    <label>Domain: <domain-picker-sk id=range .state=${ele.domain} @domain-changed=${ele._rangeChange} force_request_type=dense></domain-picker-sk></label>
    <div class=running>
      <button class=action ?disabled=${ele._notHasQuery()} @click=${ele._run}>Run</button>
      <spinner-sk ?active=${ele._isRunning}></spinner-sk>
      <span>${ele._runningStatus}</span>
    </div>
  </div>
  <hr>

  <dialog @open-keys=${ele._openKeys}>
  <cluster-summary2-sk @triaged=${ele._triaged} .full_summary=${ele._dialog_state.full_summary} .triage=${ele._dialog_state.triage}></cluster-summary2-sk>
  <div class=buttons>
    <button @click=${ele._close}>Close</button>
  </div>
  </dialog>

  <table @start-triage=${ele._triage_start}>
    <tr>
      <th>Commit</th>
      <th colspan=2>Regressions</th>
    </tr>
    <tr>
      ${_tableHeader(ele)}
    </tr>
    ${_tableRows(ele)}
  </table>
`;

window.customElements.define('cluster-lastn-page-sk', class extends ElementSk {
  constructor() {
    super(template);
  }

  connectedCallback() {
    super.connectedCallback();
    this._render();
  }

});
