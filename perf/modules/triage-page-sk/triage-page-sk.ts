/**
 * @module module/triage-page-sk
 * @description <h2><code>triage-page-sk</code></h2>
 *
 * Allows triaging clusters.
 *
 * TODO(jcgregorio) Needs working demo page and tests.
 *
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { equals, deepCopy } from '../../../infra-sk/modules/object';
import { fromObject } from '../../../infra-sk/modules/query';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { stateReflector } from '../../../infra-sk/modules/stateReflector';
import { HintableObject } from '../../../infra-sk/modules/hintable';
import { errorMessage } from '../errorMessage';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import {
  RegressionRangeRequest,
  RegressionRow,
  Subset,
  RegressionRangeResponse,
  FullSummary,
  Regression,
  FrameResponse,
  ClusterSummary,
  TriageRequest,
  TriageResponse,
} from '../json';

import '../../../elements-sk/modules/spinner-sk';

import '../cluster-summary2-sk';
import '../commit-detail-sk';
import '../day-range-sk';
import '../triage-status-sk';
import { TriageStatusSkStartTriageEventDetails } from '../triage-status-sk/triage-status-sk';
import {
  ClusterSummary2SkTriagedEventDetail,
  ClusterSummary2SkOpenKeysEventDetail,
  ClusterSummary2Sk,
} from '../cluster-summary2-sk/cluster-summary2-sk';
import { DayRangeSkChangeDetail } from '../day-range-sk/day-range-sk';
import { handleKeyboardShortcut, KeyboardShortcutHandler } from '../common/keyboard-shortcuts';
import '../keyboard-shortcuts-help-sk/keyboard-shortcuts-help-sk';
import { KeyboardShortcutsHelpSk } from '../keyboard-shortcuts-help-sk/keyboard-shortcuts-help-sk';
import '../../../elements-sk/modules/icons/help-icon-sk';

function _full_summary(frame: FrameResponse, summary: ClusterSummary): FullSummary {
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
  filter: string; // Legacy query parameter alias for alert_filter.
  alert_filter: string;
}

interface ValueOptions {
  value: string;
  title: string;
  display: string;
}

export class TriagePageSk extends ElementSk implements KeyboardShortcutHandler {
  onOpenHelp(): void {
    const help = this.querySelector('keyboard-shortcuts-help-sk') as KeyboardShortcutsHelpSk;
    help.open();
  }

  private state: State;

  private triageInProgress: boolean;

  private refreshRangeInProgress: boolean;

  private statusIntervalID: number;

  private firstConnect: boolean;

  private reg: RegressionRangeResponse;

  private dialogState: Partial<TriageStatusSkStartTriageEventDetails> = {};

  private lastState: Partial<State> = {};

  private dialog: HTMLDialogElement | null = null;

  private allFilterOptions: ValueOptions[] = [];

  constructor() {
    super(TriagePageSk.template);
    const now = Math.floor(Date.now() / 1000);

    // The state to reflect to the URL, also the body of the POST request
    // we send to /_/reg/.
    this.state = {
      begin: now - 2 * 7 * 24 * 60 * 60, // 2 weeks.
      end: now,
      subset: 'untriaged',
      alert_filter: 'ALL',
      filter: '',
    };

    this.reg = {
      header: [],
      table: [],
      categories: [],
    };

    this.allFilterOptions = [];

    this.triageInProgress = false;

    this.refreshRangeInProgress = false;

    // The ID of the setInterval that is updating _currentClusteringStatus.
    this.statusIntervalID = 0;

    this.firstConnect = false;
  }

  private static template = (ele: TriagePageSk) => html`
    <header>
      <details>
        <summary>Filter</summary>
        <h3>Which commits to display.</h3>
        <select @input=${ele.commitsChange}>
          <option
            ?selected=${ele.state.subset === 'all'}
            value="all"
            title="Show results for all commits in the time range.">
            All
          </option>
          <option
            ?selected=${ele.state.subset === 'regressions'}
            value="regressions"
            title="Show only the commits with regressions in the given time range regardless of triage status.">
            Regressions
          </option>
          <option
            ?selected=${ele.state.subset === 'untriaged'}
            value="untriaged"
            title="Show only commits with untriaged regressions in the given time range.">
            Untriaged
          </option>
        </select>

        <h3>Which alerts to display.</h3>

        <select @input=${ele.filterChange}>
          ${TriagePageSk.allFilters(ele)}
        </select>
      </details>
      <details>
        <summary>Range</summary>
        <day-range-sk
          @day-range-change=${ele.rangeChange}
          begin=${ele.state.begin}
          end=${ele.state.end}></day-range-sk>
      </details>
      <button
        class="action"
        @click=${ele.onOpenHelp}
        title="Keyboard Shortcuts"
        style="margin-left: auto;">
        <help-icon-sk></help-icon-sk>
      </button>
    </header>
    <spinner-sk ?active=${ele.triageInProgress || ele.refreshRangeInProgress}></spinner-sk>

    <dialog id="triage-dialog">
      <cluster-summary2-sk
        @open-keys=${ele.openKeys}
        @triaged=${ele.triaged}
        .full_summary=${ele.dialogState!.full_summary}
        .triage=${ele.dialogState!.triage}
        .alert=${ele.dialogState!.alert}></cluster-summary2-sk>
      <div class="buttons">
        <button @click=${ele.close}>Close</button>
      </div>
    </dialog>
    <keyboard-shortcuts-help-sk .handler=${ele}></keyboard-shortcuts-help-sk>

    <table @start-triage=${ele.triage_start}>
      <tr>
        <th>Commit</th>
        ${TriagePageSk.headers(ele)}
      </tr>
      <tr>
        <th></th>
        ${TriagePageSk.subHeaders(ele)}
      </tr>
      ${TriagePageSk.rows(ele)}
    </table>
  `;

  private static rows = (ele: TriagePageSk) =>
    ele.reg!.table!.map(
      (row, rowIndex) => html`
        <tr>
          <td class="fixed">
            <commit-detail-sk .cid=${row!.cid}></commit-detail-sk>
          </td>
          ${TriagePageSk.columns(ele, row!, rowIndex)}
        </tr>
      `
    );

  private static columns = (ele: TriagePageSk, row: RegressionRow, rowIndex: number) =>
    row.columns!.map((col, colIndex) => {
      const ret = [];

      if (ele.stepDownAt(colIndex)) {
        ret.push(html`
          <td class="cluster">${TriagePageSk.lowCell(ele, rowIndex, col!, colIndex)}</td>
        `);
      }

      if (ele.stepUpAt(colIndex)) {
        ret.push(html`
          <td class="cluster">${TriagePageSk.highCell(ele, rowIndex, col!, colIndex)}</td>
        `);
      }

      if (ele.notBoth(colIndex)) {
        ret.push(html` <td></td> `);
      }
      return ret;
    });

  private static lowCell = (
    ele: TriagePageSk,
    rowIndex: number,
    col: Regression,
    colIndex: number
  ) => {
    if (col && col.low) {
      return html`
        <triage-status-sk
          .alert=${ele.alertAt(colIndex)}
          .cluster_type=${'low'}
          .full_summary=${_full_summary(col.frame!, col.low)}
          .triage=${col.low_status}></triage-status-sk>
      `;
    }
    return html`
      <a
        title="No clusters found."
        href="/g/c/${ele.hashFrom(rowIndex)}?query=${ele.encQueryFrom(colIndex)}">
        ∅
      </a>
    `;
  };

  private static highCell = (
    ele: TriagePageSk,
    rowIndex: number,
    col: Regression,
    colIndex: number
  ) => {
    if (col && col.high) {
      return html`
        <triage-status-sk
          .alert=${ele.alertAt(colIndex)}
          .cluster_type=${'high'}
          .full_summary=${_full_summary(col.frame!, col.high)}
          .triage=${col.high_status}></triage-status-sk>
      `;
    }
    return html`
      <a
        title="No clusters found."
        href="/g/c/${ele.hashFrom(rowIndex)}?query=${ele.encQueryFrom(colIndex)}">
        ∅
      </a>
    `;
  };

  private static subHeaders = (ele: TriagePageSk) =>
    ele.reg.header!.map((_, index) => {
      const ret = [];
      if (ele.stepDownAt(index)) {
        ret.push(html` <th>Low</th> `);
      }
      if (ele.stepUpAt(index)) {
        ret.push(html` <th>High</th> `);
      }
      // If we have only one of High or Low we stuff in an empty th to match
      // colspan=2 above.
      if (ele.notBoth(index)) {
        ret.push(html` <th></th> `);
      }
      return ret;
    });

  private static headers = (ele: TriagePageSk) =>
    ele.reg.header!.map((item) => {
      let displayName = item!.display_name;
      if (!item!.display_name) {
        displayName = item!.query.slice(0, 10);
      }
      // The colspan=2 is important since we will have two columns under each
      // header, one for high and one for low.
      return html`
        <th colspan="2">
          <a href="/a/?${item!.id_as_string}">${displayName}</a>
        </th>
      `;
    });

  private static allFilters = (ele: TriagePageSk) =>
    ele.allFilterOptions.map(
      (o) => html`
        <option ?selected=${ele.state.alert_filter === o.value} value=${o.value} title=${o.title}>
          ${o.display}
        </option>
      `
    );

  connectedCallback(): void {
    super.connectedCallback();
    if (this.firstConnect) {
      return;
    }
    this.firstConnect = true;

    this._render();
    this.dialog = this.querySelector('dialog');
    this.stateHasChanged = stateReflector(
      () => this.state as unknown as HintableObject,
      (state) => {
        this.state = state as unknown as State;
        // Support the legacy query parameter.
        if (this.state.filter) {
          this.state.alert_filter = this.state.filter;
        }
        this._render();
        this.updateRange();
      }
    );
    window.addEventListener('keydown', this.keyDown);
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    window.removeEventListener('keydown', this.keyDown);
  }

  private keyDown = (e: KeyboardEvent) => {
    if (!this.dialog || !this.dialog.open) {
      return;
    }
    // Ignore if typing in input
    if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) {
      return;
    }

    handleKeyboardShortcut(e, this);
  };

  onTriagePositive(): void {
    const clusterSummary = this.querySelector('cluster-summary2-sk') as ClusterSummary2Sk;
    if (clusterSummary) {
      clusterSummary.triage = { ...clusterSummary.triage, status: 'positive' };
      clusterSummary.update();
    }
  }

  onTriageNegative(): void {
    const clusterSummary = this.querySelector('cluster-summary2-sk') as ClusterSummary2Sk;
    if (clusterSummary) {
      clusterSummary.triage = { ...clusterSummary.triage, status: 'negative' };
      clusterSummary.update();
    }
  }

  onOpenReport(): void {
    const clusterSummary = this.querySelector('cluster-summary2-sk') as ClusterSummary2Sk;
    if (clusterSummary) {
      clusterSummary.openShortcut();
    }
  }

  // eslint-disable-next-line @typescript-eslint/no-empty-function
  private stateHasChanged = () => {};

  private commitsChange(e: InputEvent) {
    this.state.subset = (e.target! as HTMLInputElement).value as Subset;
    this.updateRange();
    this.stateHasChanged();
  }

  private filterChange(e: InputEvent) {
    this.state.alert_filter = (e.target! as HTMLInputElement).value;
    this.updateRange();
    this.stateHasChanged();
  }

  private triage_start = (e: CustomEvent<TriageStatusSkStartTriageEventDetails>) => {
    this.dialogState = e.detail;
    this._render();
    const dialog = this.querySelector<HTMLDialogElement>('#triage-dialog');
    if (dialog) {
      this.dialog = dialog;
      try {
        if (!this.dialog.open) {
          this.dialog.showModal();
        }
      } catch (err) {
        errorMessage(`Failed to open dialog: ${err}`);
        // Fallback to non-modal open if showModal fails
        this.dialog.open = true;
      }
    }
  };

  private triaged(e: CustomEvent<ClusterSummary2SkTriagedEventDetail>) {
    e.stopPropagation();
    const body: TriageRequest = {
      cid: e.detail.columnHeader.offset,
      triage: e.detail.triage,
      alert: this.dialogState!.alert!,
      cluster_type: this.dialogState!.cluster_type!,
    };
    this.dialog!.close();
    this._render();
    if (this.triageInProgress) {
      errorMessage('A triage request is in progress.');
      return;
    }
    this.triageInProgress = true;
    fetch('/_/triage/', {
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then((json: TriageResponse) => {
        this.triageInProgress = false;
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
        this.triageInProgress = false;
        this._render();
      });
  }

  private close() {
    this.dialog!.close();
  }

  private stepUpAt(index: number) {
    const dir = this.reg.header![index]!.direction;
    return dir === 'UP' || dir === 'BOTH';
  }

  private stepDownAt(index: number) {
    const dir = this.reg.header![index]!.direction;
    return dir === 'DOWN' || dir === 'BOTH';
  }

  private notBoth(index: number) {
    return this.reg.header![index]!.direction !== 'BOTH';
  }

  private alertAt(index: number) {
    return this.reg.header![index];
  }

  private encQueryFrom(colIndex: number) {
    return encodeURIComponent(this.reg.header![colIndex]!.query);
  }

  private hashFrom(rowIndex: number) {
    return this.reg.table![rowIndex]!.cid!.offset;
  }

  private openKeys(e: CustomEvent<ClusterSummary2SkOpenKeysEventDetail>) {
    const query = {
      keys: e.detail.shortcut,
      begin: e.detail.begin,
      end: e.detail.end,
      xbaroffset: e.detail.xbar.offset,
      num_commits: 50,
      request_type: 1,
    };
    window.open(`/ e /? ${fromObject(query)} `, '_blank');
  }

  private rangeChange(e: CustomEvent<DayRangeSkChangeDetail>) {
    this.state.begin = Math.floor(e.detail.begin);
    this.state.end = Math.floor(e.detail.end);
    this.stateHasChanged();
    this.updateRange();
  }

  private updateRange() {
    if (this.refreshRangeInProgress) {
      return;
    }
    if (
      equals(this.lastState! as unknown as HintableObject, this.state as unknown as HintableObject)
    ) {
      return;
    }
    this.lastState = deepCopy(this.state);
    this.refreshRangeInProgress = true;
    this._render();
    const body: RegressionRangeRequest = this.state;
    fetch('/_/reg/', {
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then((json: RegressionRangeResponse) => {
        this.refreshRangeInProgress = false;
        this.reg = json;
        this.calc_all_filter_options();
        this._render();
      })
      .catch((msg) => {
        if (msg) {
          errorMessage(msg, 10000);
        }
        this.refreshRangeInProgress = false;
        this._render();
      });
  }

  private calc_all_filter_options() {
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
    if (this.reg && this.reg.categories) {
      this.reg.categories.forEach((cat) => {
        const displayName = cat || '(default)';
        opts.push({
          value: `cat:${cat} `,
          title: `Show only the alerts in the ${displayName} category.`,
          display: `Category: ${displayName} `,
        });
      });
    }
    this.allFilterOptions = opts;
  }
}

define('triage-page-sk', TriagePageSk);
