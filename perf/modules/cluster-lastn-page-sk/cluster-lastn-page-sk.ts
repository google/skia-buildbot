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
import {
  UIDomain,
  ParamSet,
  Alert,
  FrameResponse,
  RegressionRow,
  Regression,
  DryRunStatus,
  Direction,
  ClusterSummary,
  StartDryRunRequest,
  StartDryRunResponse,
} from '../json';
import { ParamSetSk } from '../../../infra-sk/modules/paramset-sk/paramset-sk';
import { AlertConfigSk } from '../alert-config-sk/alert-config-sk';
import { HintableObject } from 'common-sk/modules/hintable';
import { TriageStatusSkStartTriageEventDetails } from '../triage-status-sk/triage-status-sk';
import { ClusterSummary2SkOpenKeysEventDetail } from '../cluster-summary2-sk/cluster-summary2-sk';
import { DomainPickerSk } from '../domain-picker-sk/domain-picker-sk';

export class ClusterLastNPageSk extends ElementSk {
  private static template = (ele: ClusterLastNPageSk) => html`
    <dialog id="alert-config-dialog">
      <alert-config-sk
        .config=${ele.state}
        .paramset=${ele.paramset}
        .key_order=${window.sk.perf.key_order}
      ></alert-config-sk>
      <div class="buttons">
        <button @click=${ele.alertClose}>Cancel</button>
        <button @click=${ele.alertAccept}>Accept</button>
      </div>
    </dialog>
    <div class="controls">
      <label>
        <h2>Alert Configuration</h2>
        <button @click=${ele.alertEdit}>
          ${ClusterLastNPageSk.configTitle(ele)}
        </button>
      </label>
      <label>
        <h2>Time Range</h2>
        <domain-picker-sk
          id="range"
          .state=${ele.domain}
          force_request_type="dense"
        ></domain-picker-sk>
      </label>
      <div class="running">
        <button
          class="action"
          ?disabled=${!ele.state!.query || !!ele.requestId}
          @click=${ele.run}
        >
          Run
        </button>
        <spinner-sk ?active=${!!ele.requestId}></spinner-sk>
        <pre class="messages">${ele.runningStatus}</pre>
      </div>
    </div>
    <hr />

    <dialog id="triage-cluster-dialog" @open-keys=${ele.openKeys}>
      <cluster-summary2-sk
        .full_summary=${ele.dialogState!.full_summary}
        .triage=${ele.dialogState!.triage}
        notriage
      ></cluster-summary2-sk>
      <div class="buttons">
        <button @click=${ele.triageClose}>Close</button>
      </div>
    </dialog>

    ${ClusterLastNPageSk.table(ele)}
  `;

  private static stepUpAt(dir: Direction) {
    return dir === 'UP' || dir === 'BOTH';
  }

  private static stepDownAt(dir: Direction) {
    return dir === 'DOWN' || dir === 'BOTH';
  }

  private static notBoth(dir: Direction) {
    return dir !== 'BOTH';
  }

  private static tableHeader = (ele: ClusterLastNPageSk) => {
    const ret = [
      html`
        <th></th>
      `,
    ];
    if (ClusterLastNPageSk.stepDownAt(ele.state!.direction)) {
      ret.push(
        html`
          <th>Low</th>
        `
      );
    }
    if (ClusterLastNPageSk.stepUpAt(ele.state!.direction)) {
      ret.push(
        html`
          <th>High</th>
        `
      );
    }
    if (ClusterLastNPageSk.notBoth(ele.state!.direction)) {
      ret.push(
        html`
          <th></th>
        `
      );
    }
    return ret;
  };

  private static fullSummary(frame: FrameResponse, summary: ClusterSummary) {
    return {
      frame,
      summary,
    };
  }

  private static low = (ele: ClusterLastNPageSk, reg: RegressionRow) => {
    if (!ClusterLastNPageSk.stepDownAt(ele.state!.direction)) {
      return html``;
    }
    if (reg.regression!.low) {
      return html`
        <td class="cluster">
          <triage-status-sk
            .alert=${ele.state}
            cluster_type="low"
            .full_summary=${ClusterLastNPageSk.fullSummary(
              reg.regression!.frame!,
              reg.regression!.low
            )}
            .triage=${reg.regression!.low_status}
          ></triage-status-sk>
        </td>
      `;
    }
    return html`
      <td class="cluster"></td>
    `;
  };

  private static high = (ele: ClusterLastNPageSk, reg: RegressionRow) => {
    if (!ClusterLastNPageSk.stepUpAt(ele.state!.direction)) {
      return html``;
    }
    if (reg.regression!.high) {
      return html`
        <td class="cluster">
          <triage-status-sk
            .alert=${ele.state}
            cluster_type="high"
            .full_summary=${ClusterLastNPageSk.fullSummary(
              reg.regression!.frame!,
              reg.regression!.high
            )}
            .triage=${reg.regression!.high_status}
          ></triage-status-sk>
        </td>
      `;
    }
    return html`
      <td class="cluster"></td>
    `;
  };

  private static filler = (ele: ClusterLastNPageSk) => {
    if (ClusterLastNPageSk.notBoth(ele.state!.direction)) {
      return html`
        <td></td>
      `;
    }
    return html``;
  };

  private static tableRows = (ele: ClusterLastNPageSk) =>
    ele._regressions.map(
      (reg) => html`
        <tr>
          <td class="fixed">
            <commit-detail-sk .cid=${reg.cid}></commit-detail-sk>
          </td>

          ${ClusterLastNPageSk.low(ele, reg)}
          ${ClusterLastNPageSk.high(ele, reg)} ${ClusterLastNPageSk.filler(ele)}
        </tr>
      `
    );

  private static configTitle = (ele: ClusterLastNPageSk) =>
    html`
      Algo: ${ele.state!.algo} - Radius: ${ele.state!.radius} - Sparse:
      ${ele.state!.sparse} - Threshold: ${ele.state!.interesting}
    `;

  private static table = (ele: ClusterLastNPageSk) => {
    if (ele.requestId && !ele._regressions.length) {
      return html`
        No regressions found yet.
      `;
    }
    return html`
      <table @start-triage=${ele.triageStart}>
        <tr>
          <th>Commit</th>
          <th colspan="2">Regressions</th>
        </tr>
        <tr>
          ${ClusterLastNPageSk.tableHeader(ele)}
        </tr>
        ${ClusterLastNPageSk.tableRows(ele)}
      </table>
    `;
  };

  // The range of commits over which we are clustering.
  private domain: UIDomain = {
    begin: 0,
    end: Math.floor(Date.now() / 1000),
    num_commits: 200,
    request_type: 1,
  };

  // The paramsets for the alert config.
  private paramset: ParamSet = {};

  // The id of the currently running request.
  private requestId: string = '';

  // The text status of the currently running request.
  private runningStatus = '';

  // The fetch in connectedCallback will fill this is with the defaults.
  private state: Alert | null = null;

  // The regressions detected from the dryrun.
  private _regressions: RegressionRow[] = [];

  private alertDialog: HTMLDialogElement | null = null;
  private triageDialog: HTMLDialogElement | null = null;
  private alertConfig: AlertConfigSk | null = null;

  // The state of the cluster-summary2-sk dialog.
  private dialogState: Partial<TriageStatusSkStartTriageEventDetails> | null = {
    full_summary: null,
    triage: undefined,
  };

  // Call this anytime something in private state is changed. Will be replaced
  // with the real function once stateReflector has been setup.
  // tslint:disable-next-line: no-empty
  private _stateHasChanged = () => {};

  constructor() {
    super(ClusterLastNPageSk.template);
  }

  connectedCallback() {
    super.connectedCallback();

    const init = fetch('/_/initpage/')
      .then(jsonOrThrow)
      .then((json: FrameResponse) => {
        this.paramset = json.dataframe!.paramset;
      });

    const alertNew = fetch('/_/alert/new')
      .then(jsonOrThrow)
      .then((json: Alert) => {
        this.state = json;
      });

    Promise.all([init, alertNew])
      .then(() => {
        this._render();
        this.alertDialog = this.querySelector('#alert-config-dialog');
        this.triageDialog = this.querySelector('#triage-cluster-dialog');
        dialogPolyfill.registerDialog(this.alertDialog!);
        dialogPolyfill.registerDialog(this.triageDialog!);
        this.alertConfig = this.querySelector('alert-config-sk');
        this._stateHasChanged = stateReflector(
          () => (this.state as unknown) as HintableObject,
          (state) => {
            this.state = (state as unknown) as Alert;
            this._render();
          }
        );
      })
      .catch(errorMessage);
  }

  private alertEdit() {
    this.alertDialog!.showModal();
  }

  private alertClose() {
    this.alertDialog!.close();
  }

  private alertAccept() {
    this.alertDialog!.close();
    this.state = this.alertConfig!.config;
    this._stateHasChanged();
    this._render();
  }

  private triageStart(e: CustomEvent<TriageStatusSkStartTriageEventDetails>) {
    this.dialogState = e.detail;
    this._render();
    this.triageDialog!.show();
  }

  private triageClose() {
    this.triageDialog!.close();
  }

  private openKeys(e: CustomEvent<ClusterSummary2SkOpenKeysEventDetail>) {
    const query = {
      keys: e.detail.shortcut,
      begin: e.detail.begin,
      end: e.detail.end,
      xbaroffset: e.detail.xbar.offset,
    };
    window.open(`/e/?${fromObject(query)}`, '_blank');
  }

  private catch(msg: string) {
    this.requestId = '';
    this.runningStatus = '';
    this._render();
    if (msg) {
      errorMessage(msg, 10000);
    }
  }

  private run() {
    if (this.requestId) {
      errorMessage('There is a pending query already running.');
      return;
    }
    this.domain = this.querySelector<DomainPickerSk>('#range')!.state;
    const body: StartDryRunRequest = {
      domain: this.domain,
      config: this.state!,
    };
    fetch('/_/dryrun/start', {
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then((json: StartDryRunResponse) => {
        this.requestId = json.id;
        this._render();
        this.checkDryRunStatus((regressions: RegressionRow[]) => {
          this._regressions = regressions;
          this._render();
        });
      })
      .catch((msg) => this.catch(msg));
  }

  private checkDryRunStatus(cb: (regressions: RegressionRow[]) => void) {
    fetch(`/_/dryrun/status/${this.requestId}`)
      .then(jsonOrThrow)
      .then((json: DryRunStatus) => {
        this.runningStatus = json.message;
        if (!json.finished) {
          window.setTimeout(() => this.checkDryRunStatus(cb), 300);
        } else {
          this.requestId = '';
        }
        // json.regressions will get filled in incrementally, so display them
        // as they arrive.
        cb(json.regressions!);
      })
      .catch((msg) => this.catch(msg));
  }
}

define('cluster-lastn-page-sk', ClusterLastNPageSk);
