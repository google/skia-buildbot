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
import {
  RegressionRangeRequest,
  RegressionRow,
  Subset,
  RegressionRangeResponse,
  TriageStatus,
  FullSummary,
  Regression,
  FrameResponse,
  ClusterSummary,
  Current,
  TriageRequest,
} from '../json';

import 'elements-sk/spinner-sk';
import 'elements-sk/styles/buttons';
import 'elements-sk/styles/select';

import '../cluster-summary2-sk';
import '../commit-detail-sk';
import '../day-range-sk';
import '../triage-status-sk';
import { HintableObject } from 'common-sk/modules/hintable';
import { TriageStatusSkStartTriageEventDetails } from '../triage-status-sk/triage-status-sk';
import {
  ClusterSummary2SkTriagedEventDetail,
  ClusterSummary2SkOpenKeysEventDetail,
} from '../cluster-summary2-sk/cluster-summary2-sk';
import { DayRangeSkChangeDetail } from '../day-range-sk/day-range-sk';

function _full_summary(
  frame: FrameResponse,
  summary: ClusterSummary
): FullSummary {
  return {
    frame,
    summary,
    triage: {
      message: '',
      status: 'untriaged',
    },
  };
}

interface State {
  begin: number;
  end: number;
  subset: Subset;
  alert_filter: string;
}

interface DialogState {
  full_summary: FullSummary | null;
  triage: TriageStatus;
}

interface ValueOptions {
  value: string;
  title: string;
  display: string;
}

export class TriagePageSk extends ElementSk {
  private static template = (ele: TriagePageSk) => html`
    <header>
      <details>
        <summary>
          <h2>Filter</h2>
        </summary>
        <h3>Which commits to display.</h3>
        <select @input=${ele._commitsChange}>
          <option
            ?selected=${ele._state.subset === 'all'}
            value="all"
            title="Show results for all commits in the time range."
            >All</option
          >
          <option
            ?selected=${ele._state.subset === 'regressions'}
            value="regressions"
            title="Show only the commits with regressions in the given time range regardless of triage status."
            >Regressions</option
          >
          <option
            ?selected=${ele._state.subset === 'untriaged'}
            value="untriaged"
            title="Show only commits with untriaged regressions in the given time range."
            >Untriaged</option
          >
        </select>

        <h3>Which alerts to display.</h3>

        <select @input=${ele._filterChange}>
          ${TriagePageSk._allFilters(ele)}
        </select>
      </details>
      <details>
        <summary>
          <h2>Range</h2>
        </summary>
        <day-range-sk
          @day-range-change=${ele._rangeChange}
          begin=${ele._state.begin}
          end=${ele._state.end}
        ></day-range-sk>
      </details>
      <details @toggle=${ele._toggleStatus}>
        <summary>
          <h2>Status</h2>
        </summary>
        <div>
          <p>The current work on detecting regressions:</p>
          <div class="status">
            ${TriagePageSk._statusItems(ele)}
          </div>
        </div>
      </details>
    </header>
    <spinner-sk
      ?active=${ele._triageInProgress || ele._refreshRangeInProgress}
    ></spinner-sk>

    <dialog>
      <cluster-summary2-sk
        @open-keys=${ele._openKeys}
        @triaged=${ele._triaged}
        .full_summary=${ele._dialog_state!.full_summary}
        .triage=${ele._dialog_state!.triage}
      >
      </cluster-summary2-sk>
      <div class="buttons">
        <button @click=${ele._close}>Close</button>
      </div>
    </dialog>

    <table @start-triage=${ele._triage_start}>
      <tr>
        <th>Commit</th>
        ${TriagePageSk._headers(ele)}
      </tr>
      <tr>
        <th></th>
        ${TriagePageSk._subHeaders(ele)}
      </tr>
      ${TriagePageSk._rows(ele)}
    </table>
  `;

  private static _rows = (ele: TriagePageSk) =>
    ele._reg!.table!.map(
      (row, rowIndex) => html` <tr>
        <td class="fixed">
          <commit-detail-sk .cid=${row.cid}></commit-detail-sk>
        </td>
        ${TriagePageSk._columns(ele, row, rowIndex)}
      </tr>`
    );

  private static _columns = (
    ele: TriagePageSk,
    row: RegressionRow,
    rowIndex: number
  ) =>
    row.columns!.map((col, colIndex) => {
      const ret = [];

      if (ele._stepDownAt(colIndex)) {
        ret.push(html`
          <td class="cluster">
            ${TriagePageSk._lowCell(ele, rowIndex, col, colIndex)}
          </td>
        `);
      }

      if (ele._stepUpAt(colIndex)) {
        ret.push(html`
          <td class="cluster">
            ${TriagePageSk._highCell(ele, rowIndex, col, colIndex)}
          </td>
        `);
      }

      if (ele._notBoth(colIndex)) {
        ret.push(html`<td></td>`);
      }
      return ret;
    });

  private static _lowCell = (
    ele: TriagePageSk,
    rowIndex: number,
    col: Regression,
    colIndex: number
  ) => {
    if (col && col.low) {
      return html`<triage-status-sk
        .alert=${ele._alertAt(colIndex)}
        .cluster_type=${'low'}
        .full_summary=${_full_summary(col.frame!, col.low)}
        .triage=${col.low_status}
      ></triage-status-sk> `;
    }
    return html`<a
      title="No clusters found."
      href="/g/c/${ele._hashFrom(rowIndex)}?query=${ele._encQueryFrom(
        colIndex
      )}"
      >∅</a
    > `;
  };

  private static _highCell = (
    ele: TriagePageSk,
    rowIndex: number,
    col: Regression,
    colIndex: number
  ) => {
    if (col && col.high) {
      return html`<triage-status-sk
        .alert=${ele._alertAt(colIndex)}
        .cluster_type=${'high'}
        .full_summary=${_full_summary(col.frame!, col.high)}
        .triage=${col.high_status}
      ></triage-status-sk> `;
    }
    return html`<a
      title="No clusters found."
      href="/g/c/${ele._hashFrom(rowIndex)}?query=${ele._encQueryFrom(
        colIndex
      )}"
      >∅</a
    > `;
  };

  private static _subHeaders = (ele: TriagePageSk) =>
    ele._reg.header!.map((_, index) => {
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

  private static _headers = (ele: TriagePageSk) =>
    ele._reg.header!.map((item) => {
      let displayName = item.display_name;
      if (!item.display_name) {
        displayName = item.query.slice(0, 10);
      }
      // The colspan=2 is important since we will have two columns under each
      // header, one for high and one for low.
      return html`<th colspan="2"
        ><a href="/a/?${item.id}">${displayName}</a></th
      >`;
    });

  private static _statusItems = (ele: TriagePageSk) =>
    ele._currentClusteringStatus.map(
      (item) => html`
        <table>
          <tr>
            <th>Alert</th>
            <td
              ><a href="/a/?${item.alert!.id}"
                >${item.alert!.display_name}</a
              ></td
            >
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
      `
    );

  private static _allFilters = (ele: TriagePageSk) =>
    ele._all_filter_options.map(
      (o) => html` <option
        ?selected=${ele._state.alert_filter === o.value}
        value=${o.value}
        title=${o.title}
        >${o.display}
      </option>`
    );

  private _state: State;
  private _triageInProgress: boolean;
  private _refreshRangeInProgress: boolean;
  private _statusIntervalID: number;
  private _firstConnect: boolean;
  private _reg: RegressionRangeResponse;
  private _dialog_state: TriageStatusSkStartTriageEventDetails | null = null;
  private _lastState: State | null = null;
  private _dialog: HTMLDialogElement | null = null;
  private _all_filter_options: ValueOptions[] = [];
  private _stateHasChanged: () => void = () => {};
  private _currentClusteringStatus: Current[] = [];

  constructor() {
    super(TriagePageSk.template);
    const now = Math.floor(Date.now() / 1000);

    // The state to reflect to the URL, also the body of the POST request
    // we send to /_/reg/.
    this._state = {
      begin: now - 2 * 7 * 24 * 60 * 60, // 2 weeks.
      end: now,
      subset: 'untriaged',
      alert_filter: 'ALL',
    };

    this._reg = {
      header: [],
      table: [],
      categories: [],
    };

    this._all_filter_options = [];

    this._triageInProgress = false;

    this._refreshRangeInProgress = false;

    // The ID of the setInterval that is updating _currentClusteringStatus.
    this._statusIntervalID = 0;

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
    dialogPolyfill.registerDialog(this.querySelector('dialog')!);
    this._stateHasChanged = stateReflector(
      () => (this._state as unknown) as HintableObject,
      (state) => {
        this._state = (state as unknown) as State;
        this._render();
        this._updateRange();
      }
    );
  }

  _commitsChange(e: InputEvent) {
    this._state.subset = (e.target! as HTMLInputElement).value as Subset;
    this._updateRange();
    this._stateHasChanged();
  }

  _filterChange(e: InputEvent) {
    this._state.alert_filter = (e.target! as HTMLInputElement).value;
    this._updateRange();
    this._stateHasChanged();
  }

  _toggleStatus(e: InputEvent) {
    if ((e.target! as HTMLDetailsElement).open) {
      this._statusIntervalID = window.setInterval(
        () => this._pollStatus(),
        5000
      );
      this._pollStatus();
    } else {
      window.clearInterval(this._statusIntervalID);
    }
  }

  _pollStatus() {
    fetch('/_/reg/current')
      .then(jsonOrThrow)
      .then((json) => {
        this._currentClusteringStatus = json;
        this._render();
      })
      .catch(errorMessage);
  }

  _triage_start(e: CustomEvent<TriageStatusSkStartTriageEventDetails>) {
    this._dialog_state = e.detail;
    this._render();
    this._dialog!.showModal();
  }

  _triaged(e: CustomEvent<ClusterSummary2SkTriagedEventDetail>) {
    e.stopPropagation();
    const body: TriageRequest = Object.assign({}, e.detail, {
      alert: this._dialog_state!.alert!,
      cluster_type: this._dialog_state!.cluster_type,
    });
    this._dialog!.close();
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
    })
      .then(jsonOrThrow)
      .then((json) => {
        this._triageInProgress = false;
        this._render();
        if (json.bug) {
          // Open the bug reporting page in a new window.
          window.open(json.bug, '_blank');
        }
      })
      .catch((msg) => {
        if (msg) {
          errorMessage(msg, 10000);
        }
        this._triageInProgress = false;
        this._render();
      });
  }

  _close() {
    this._dialog!.close();
  }

  _stepUpAt(index: number) {
    const dir = this._reg.header![index].direction;
    return dir === 'UP' || dir === 'BOTH';
  }

  _stepDownAt(index: number) {
    const dir = this._reg.header![index].direction;
    return dir === 'DOWN' || dir === 'BOTH';
  }

  _notBoth(index: number) {
    return this._reg.header![index].direction !== 'BOTH';
  }

  _alertAt(index: number) {
    return this._reg.header![index];
  }

  _encQueryFrom(colIndex: number) {
    return encodeURIComponent(this._reg.header![colIndex].query);
  }

  _hashFrom(rowIndex: number) {
    return this._reg.table![rowIndex].cid!.hash;
  }

  _openKeys(e: CustomEvent<ClusterSummary2SkOpenKeysEventDetail>) {
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

  _rangeChange(e: CustomEvent<DayRangeSkChangeDetail>) {
    this._state.begin = Math.floor(e.detail.begin);
    this._state.end = Math.floor(e.detail.end);
    this._stateHasChanged();
    this._updateRange();
  }

  _updateRange() {
    if (this._refreshRangeInProgress) {
      return;
    }
    if (
      equals(
        (this._lastState! as unknown) as HintableObject,
        (this._state as unknown) as HintableObject
      )
    ) {
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
    })
      .then(jsonOrThrow)
      .then((json) => {
        this._refreshRangeInProgress = false;
        this._reg = json;
        this._calc_all_filter_options();
        this._render();
      })
      .catch((msg) => {
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
        title:
          "Show only the alerts owned by the logged in user (or all alerts if the user doesn't own any alerts).",
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
}

define('triage-page-sk', TriagePageSk);
