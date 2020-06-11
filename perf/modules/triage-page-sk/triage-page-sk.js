/**
 * @module module/triage-page-sk
 * @description <h2><code>triage-page-sk</code></h2>
 *
 * Allows triaging clusters.
 *
 */
import dialogPolyfill from 'dialog-polyfill';
import { define } from 'elements-sk/define';
import { equals, deepCopy } from 'common-sk/modules/object';
import { errorMessage } from 'elements-sk/errorMessage';
import { fromObject } from 'common-sk/modules/query';
import { html } from 'lit-html';
import { jsonOrThrow } from 'common-sk/modules/jsonOrThrow';
import { stateReflector } from 'common-sk/modules/stateReflector';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import 'elements-sk/spinner-sk';
import 'elements-sk/styles/buttons';
import 'elements-sk/styles/select';

import '../cluster-summary2-sk';
import '../commit-detail-sk';
import '../day-range-sk';
import '../triage-status-sk';

const _allFilters = (ele) => ele._all_filter_options.map(
  (o) => html`
    <option
      ?selected=${ele._state.alert_filter === o.value}
      value=${o.value}
      title=${o.title}
      >${o.display}
    </option>`,
);

const _statusItems = (ele) => ele._currentClusteringStatus.map((item) => html`
  <table>
    <tr>
      <th>Alert</th>
      <td><a href='/a/?${item.alert.id}'>${item.alert.display_name}</a></td>
    </tr>
    <tr>
      <th>Commit</th>
      <td><commit-detail-sk .cid=${item.commit}></commit-detail-sk></td>
    </tr>
    <tr>
      <th>Step</th>
      <td>${item.message}</td>
    </tr>
  </table>
`);

const _headers = (ele) => ele._reg.header.map((item) => {
  let displayName = item.display_name;
  if (!item.display_name) {
    displayName = item.query.slice(0, 10);
  }
  // The colspan=2 is important since we will have two columns under each
  // header, one for high and one for low.
  return html`<th colspan=2><a href='/a/?${item.id}'>${displayName}</a></th>`;
});

const _subHeaders = (ele) => ele._reg.header.map((_, index) => {
  const ret = [];
  if (ele._stepDownAt(index)) {
    ret.push(html`<th>Low</th>`);
  }
  if (ele._stepUpAt(index)) {
    ret.push(html`<th>High</th>`);
  }
  // If we have only one of High or Low we stuff in an empty th to match
  // colspan=2 above.
  if (ele._notBoth(index)) {
    ret.push(html`<th></th>`);
  }
  return ret;
});

function _full_summary(frame, summary) {
  return {
    frame: frame,
    summary: summary,
  };
}

const _lowCell = (ele, rowIndex, col, colIndex) => {
  if (col && col.low) {
    return html`<triage-status-sk
                  .alert=${ele._alertAt(colIndex)}
                  .cluster_type=${'low'}
                  .full_summary=${_full_summary(col.frame, col.low)}
                  .triage=${col.low_status}></triage-status-sk> `;
  }
  return html`<a
                  title='No clusters found.'
                  href='/g/c/${ele._hashFrom(rowIndex)}?query=${ele._encQueryFrom(colIndex)}'>∅</a> `;
};

const _highCell = (ele, rowIndex, col, colIndex) => {
  if (col && col.high) {
    return html`<triage-status-sk
                  .alert=${ele._alertAt(colIndex)}
                  .cluster_type=${'high'}
                  .full_summary=${_full_summary(col.frame, col.high)}
                  .triage=${col.high_status}></triage-status-sk> `;
  }
  return html`<a
                  title='No clusters found.'
                  href='/g/c/${ele._hashFrom(rowIndex)}?query=${ele._encQueryFrom(colIndex)}'>∅</a> `;
};

const _columns = (ele, row, rowIndex) => row.columns.map((col, colIndex) => {
  const ret = [];

  if (ele._stepDownAt(colIndex)) {
    ret.push(html`
      <td class=cluster>
        ${_lowCell(ele, rowIndex, col, colIndex)}
      </td>
    `);
  }

  if (ele._stepUpAt(colIndex)) {
    ret.push(html`
      <td class=cluster>
        ${_highCell(ele, rowIndex, col, colIndex)}
      </td>
    `);
  }

  if (ele._notBoth(colIndex)) {
    ret.push(html`<td></td>`);
  }
  return ret;
});

const _rows = (ele) => ele._reg.table.map((row, rowIndex) => html`
  <tr>
    <td class=fixed>
      <commit-detail-sk .cid=${row.cid}></commit-detail-sk>
    </td>
    ${_columns(ele, row, rowIndex)}
  </tr>`);

const template = (ele) => html`
  <header>
    <details>
      <summary>
        <h2>Filter</h2>
      </summary>
      <h3>Which commits to display.</h3>
      <select
        @input=${ele._commitsChange}
        >
        <option
          ?selected=${ele._state.subset === 'all'}
          value=all
          title='Show results for all commits in the time range.'>All</option>
        <option
          ?selected=${ele._state.subset === 'flagged'}
          value=flagged
          title='Show only the commits with regressions in the given time range regardless of triage status.'>Regressions</option>
        <option
          ?selected=${ele._state.subset === 'untriaged'}
          value=untriaged
          title='Show only commits with untriaged regressions in the given time range.'>Untriaged</option>
      </select>

      <h3>Which alerts to display.</h3>

      <select @input=${ele._filterChange}>
        ${_allFilters(ele)}
      </select>
    </details>
    <details>
      <summary>
        <h2>Range</h2>
      </summary>
      <day-range-sk @day-range-change=${ele._rangeChange} begin=${ele._state.begin} end=${ele._state.end}></day-range-sk>
    </details>
    <details @toggle=${ele._toggleStatus}>
      <summary>
        <h2>Status</h2>
      </summary>
      <div>
        <p>The current work on detecting regressions:</p>
        <div class=status>
          ${_statusItems(ele)}
        </div>
      </div>
    </details>
  </header>
  <spinner-sk ?active=${ele._triageInProgress || ele._refreshRangeInProgress}></spinner-sk>

  <dialog>
    <cluster-summary2-sk
      @open-keys=${ele._openKeys}
      @triaged=${ele._triaged}
      .full_summary=${ele._dialog_state.full_summary}
      .triage=${ele._dialog_state.triage}>
    </cluster-summary2-sk>
    <div class=buttons>
      <button @click=${ele._close}>Close</button>
    </div>
  </dialog>

  <table @start-triage=${ele._triage_start}>
    <tr>
      <th>Commit</th>
      ${_headers(ele)}
    </tr>
    <tr>
      <th></th>
      ${_subHeaders(ele)}
    </tr>
    ${_rows(ele)}
  </table>
`;

define('triage-page-sk', class extends ElementSk {
  constructor() {
    super(template);
    const now = Math.floor(Date.now() / 1000);

    // The state to reflect to the URL, also the body of the POST request
    // we send to /_/reg/.
    this._state = {
      begin: now - 2 * 7 * 24 * 60 * 60, // 2 weeks.
      end: now,
      subset: 'untriaged',
      filter: 'ALL',
    };

    this._reg = {
      header: [],
      table: [],
    };

    this._all_filter_options = [];

    this._triageInProgress = false;

    this._refreshRangeInProgress = false;

    this._currentClusteringStatus = [];

    // The ID of the setInterval that is updating _currentClusteringStatus.
    this._statusIntervalID = 0;

    this._dialog_state = {
      full_summary: {},
      triage: {},
    };

    this._lastState = {};

    this._firstConnect = false;
  }

  connectedCallback() {
    super.connectedCallback();
    if (this._firstConnect) {
      return;
    }
    this._firstConnect = true;

    this._render();
    this._dialog = this.querySelector('triage-page-sk > dialog');
    dialogPolyfill.registerDialog(this.querySelector('dialog'));
    this._stateHasChanged = stateReflector(() => this._state, (state) => {
      this._state = state;
      // For backwards compatibility with existing URLs that used 'filter'.
      if (this._state.filter) {
        this._state.alert_filter = this._state.filter;
      }
      this._render();
      this._updateRange();
    });
  }

  _commitsChange(e) {
    this._state.subset = e.target.value;
    this._updateRange();
    this._stateHasChanged();
  }

  _filterChange(e) {
    this._state.alert_filter = e.target.value;
    this._updateRange();
    this._stateHasChanged();
  }

  _toggleStatus(e) {
    if (e.target.open) {
      this._statusIntervalID = window.setInterval(() => this._pollStatus(), 5000);
      this._pollStatus();
    } else {
      window.clearInterval(this._statusIntervalID);
    }
  }

  _pollStatus() {
    fetch('/_/reg/current').then(jsonOrThrow).then((json) => {
      this._currentClusteringStatus = json;
      this._render();
    }).catch(errorMessage);
  }

  _triage_start(e) {
    this._dialog_state = e.detail;
    this._render();
    this._dialog.showModal();
  }

  _triaged(e) {
    e.stopPropagation();
    const body = { ...e.detail };
    body.alert = this._dialog_state.alert;
    body.cluster_type = this._dialog_state.cluster_type;
    this._dialog.close();
    this._render();
    if (this._triageInProgress) {
      errorMessage('A triage request is in progress.');
      return;
    }
    this._triageInProgress = true;
    fetch('/_/triage/', {
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
      },
    }).then(jsonOrThrow).then((json) => {
      this._triageInProgress = false;
      this._render();
      if (json.bug) {
        // Open the bug reporting page in a new window.
        window.open(json.bug, '_blank');
      }
    }).catch((msg) => {
      if (msg) {
        errorMessage(msg, 10000);
      }
      this._triageInProgress = false;
      this._render();
    });
  }

  _close() {
    this._dialog.close();
  }

  _stepUpAt(index) {
    const dir = this._reg.header[index].direction;
    return dir === 'UP' || dir === 'BOTH';
  }

  _stepDownAt(index) {
    const dir = this._reg.header[index].direction;
    return dir === 'DOWN' || dir === 'BOTH';
  }

  _notBoth(index) {
    return this._reg.header[index].direction !== 'BOTH';
  }

  _alertAt(index) {
    return this._reg.header[index];
  }

  _encQueryFrom(colIndex) {
    return encodeURIComponent(this._reg.header[colIndex].query);
  }

  _hashFrom(rowIndex) {
    return this._reg.table[rowIndex].cid.hash;
  }

  _openKeys(e) {
    const query = {
      keys: e.detail.shortcut,
      begin: e.detail.begin,
      end: e.detail.end,
      xbaroffset: e.detail.xbar.offset,
      num_commits: 50,
      request_type: 1,
    };
    window.open(`/e/?${fromObject(query)}`, '_blank');
  }

  _rangeChange(e) {
    this._state.begin = Math.floor(e.detail.begin);
    this._state.end = Math.floor(e.detail.end);
    this._stateHasChanged();
    this._updateRange();
  }

  _updateRange() {
    if (this._refreshRangeInProgress) {
      return;
    }
    if (equals(this._lastState, this._state)) {
      return;
    }
    this._lastState = deepCopy(this._state);
    this._refreshRangeInProgress = true;
    this._render();
    fetch('/_/reg/', {
      method: 'POST',
      body: JSON.stringify(this._state),
      headers: {
        'Content-Type': 'application/json',
      },
    }).then(jsonOrThrow).then((json) => {
      this._refreshRangeInProgress = false;
      this._reg = json;
      this._calc_all_filter_options();
      this._render();
    }).catch((msg) => {
      if (msg) {
        errorMessage(msg, 10000);
      }
      this._refreshRangeInProgress = false;
      this._render();
    });
  }

  _calc_all_filter_options() {
    const opts = [
      {
        value: 'ALL',
        title: 'Show all alerts.',
        display: 'Show all alerts.',
      },
      {
        value: 'OWNER',
        title: 'Show only the alerts owned by the logged in user (or all alerts if the user doesn\'t own any alerts).',
        display: 'Show alerts you own.',
      },
    ];
    if (this._reg && this._reg.categories) {
      this._reg.categories.forEach((cat) => {
        const displayName = cat || '(default)';
        opts.push({
          value: `cat:${cat}`,
          title: `Show only the alerts in the ${displayName} category.`,
          display: `Category: ${displayName}`,
        });
      });
    }
    this._all_filter_options = opts;
  }
});
