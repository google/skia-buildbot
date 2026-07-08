/**
 * @module module/cluster-lastn-page-sk
 * @description <h2><code>cluster-lastn-page-sk</code></h2>
 *
 *  Allows trying out an alert by clustering over a range of commits.
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { fromObject } from '../../../infra-sk/modules/query';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { stateReflector } from '../../../infra-sk/modules/stateReflector';
import { HintableObject } from '../../../infra-sk/modules/hintable';
import { SpinnerSk } from '../../../elements-sk/modules/spinner-sk/spinner-sk';
import { errorMessage } from '../errorMessage';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';

import '../../../elements-sk/modules/spinner-sk';

import '../cluster-summary2-sk';
import '../commit-detail-sk';
import '../triage-status-sk';
import '../alert-config-sk';
import '../domain-picker-sk';
import '../sheriff-configs-dry-run-sk';
import '../anomalies-table-sk';
import { SheriffConfigsDryRunSk } from '../sheriff-configs-dry-run-sk/sheriff-configs-dry-run-sk';
import '../../../elements-sk/modules/icons/close-icon-sk';
import { Anomaly } from '../json';
import {
  Alert,
  FrameResponse,
  RegressionAtCommit,
  Direction,
  ClusterSummary,
  AlertUpdateResponse,
  progress,
  ReadOnlyParamSet,
} from '../json';
import { DomainPickerState } from '../domain-picker-sk/domain-picker-sk';
import { AlertConfigSk } from '../alert-config-sk/alert-config-sk';
import { TriageStatusSkStartTriageEventDetails } from '../triage-status-sk/triage-status-sk';
import { ClusterSummary2SkOpenKeysEventDetail } from '../cluster-summary2-sk/cluster-summary2-sk';
import { DomainPickerSk } from '../domain-picker-sk/domain-picker-sk';
import { messagesToErrorString, startRequest } from '../progress/progress';
import { LoggedIn } from '../../../infra-sk/modules/alogin-sk/alogin-sk';

export class ClusterLastNPageSk extends ElementSk {
  // The range of commits over which we are clustering.
  private domain: DomainPickerState = {
    begin: 0,
    end: Math.floor(Date.now() / 1000),
    num_commits: 200,
    request_type: 1,
  };

  // True if the Alert is being saved to the database.
  private writingAlert: boolean = false;

  // Mode for dry run: 'legacy' or 'sheriff'
  private mode: 'legacy' | 'sheriff' = 'sheriff';

  // The paramsets for the alert config.
  private paramset = ReadOnlyParamSet({});

  // The id of the currently running request.
  private requestId: string = '';

  // The text status of the currently running request.
  private runningStatus = '';

  // The fetch in connectedCallback will fill this is with the defaults.
  private state: Alert | null = null;

  // The regressions detected from the dryrun.
  private regressions: (RegressionAtCommit | null)[] = [];

  private detectedAnomalies: Anomaly[] = [];

  private get alertDialog() {
    return this.querySelector<HTMLDialogElement>('#alert-config-dialog');
  }

  private get triageDialog() {
    return this.querySelector<HTMLDialogElement>('#triage-cluster-dialog');
  }

  private get alertConfig() {
    return this.querySelector<AlertConfigSk>('alert-config-sk');
  }

  private get sheriffConfig() {
    return this.querySelector<SheriffConfigsDryRunSk>('sheriff-configs-dry-run-sk');
  }

  private get runSpinner() {
    return this.querySelector<SpinnerSk>('#run-spinner');
  }

  // Email of the logged in user. Empty string otherwise
  user_id: string = '';

  // The state of the cluster-summary2-sk dialog.
  private dialogState: Partial<TriageStatusSkStartTriageEventDetails> | null = {
    full_summary: null,
    triage: undefined,
  };

  /** Is true if the previous Run has returned an error. */
  private hasError: boolean = false;

  constructor() {
    super(ClusterLastNPageSk.template);
    if (window.perf.demo) {
      this.domain.end = Math.floor(new Date(2020, 4, 1).valueOf() / 1000);
    }
  }

  private static template = (ele: ClusterLastNPageSk) => html`
    <dialog id="alert-config-dialog">
      <alert-config-sk
        .config=${ele.state}
        .paramset=${ele.paramset}
        .key_order=${window.perf.key_order}></alert-config-sk>
      <button id="clusterCloseIcon" @click=${ele.alertClose}>
        <close-icon-sk></close-icon-sk>
      </button>
      <div class="buttons">
        <button @click=${ele.alertClose}>Cancel</button>
        <button @click=${ele.alertAccept}>Accept</button>
      </div>
    </dialog>
    <div class="controls">
      <div style="margin-bottom: 2em;">
        <label>
          <input
            type="radio"
            name="mode"
            value="sheriff"
            ?checked=${ele.mode === 'sheriff'}
            @change=${() => {
              ele.mode = 'sheriff';
              ele._render();
            }} />
          Sheriff Config
        </label>
        <label style="margin-left: 1em;">
          <input
            type="radio"
            name="mode"
            value="legacy"
            ?checked=${ele.mode === 'legacy'}
            @change=${() => {
              ele.mode = 'legacy';
              ele._render();
            }} />
          Legacy Alert Config
        </label>
      </div>

      ${ele.mode === 'legacy'
        ? html`
            <p>
              Use this page to test out an Alert configuration. Configure the Alert by pressing the
              button below.
            </p>
            <button id="config-button" @click=${ele.alertEdit}>
              ${ClusterLastNPageSk.configTitle(ele)}
            </button>
          `
        : html` <sheriff-configs-dry-run-sk></sheriff-configs-dry-run-sk> `}

      <p>You can optionally change the range of commits over which the Alert is run:</p>
      <details>
        <summary>Range</summary>
        <domain-picker-sk
          id="range"
          .state=${ele.domain}
          force_request_type="dense"></domain-picker-sk>
      </details>
      <p>Once configured, you can run the Alert and see the regressions it detects.</p>
      <div class="running">
        <button
          id="run-button"
          class="action"
          ?disabled=${ele.mode === 'legacy'
            ? !ele.state!.query || !!ele.requestId
            : !!ele.requestId}
          @click=${ele.run}>
          Run
        </button>
        <spinner-sk id="run-spinner"></spinner-sk>
        <pre class="messages ${ClusterLastNPageSk.classIfError(ele.hasError)}">
${ele.runningStatus}</pre
        >
      </div>
      ${ele.mode === 'legacy'
        ? html`
            <div class="saving">
              <p>Once satisfied with the Alert you can save it to be run periodically.</p>
              <button @click=${ele.writeAlert} class="action" ?disabled=${!ele.state!.query}>
                ${ClusterLastNPageSk.writeAlertTitle(ele)}
              </button>
              <spinner-sk ?active=${ele.writingAlert}></spinner-sk>
            </div>
          `
        : ''}
    </div>
    <hr />

    <dialog id="triage-cluster-dialog" @open-keys=${ele.openKeys}>
      <cluster-summary2-sk
        .full_summary=${ele.dialogState!.full_summary}
        .triage=${ele.dialogState!.triage}
        .alert=${ele.state}
        notriage></cluster-summary2-sk>
      <div class="buttons">
        <button @click=${ele.triageClose}>Close</button>
      </div>
    </dialog>

    ${ele.mode === 'legacy'
      ? ClusterLastNPageSk.table(ele)
      : html`
          <div style="margin-top: 2em;" ?hidden=${!!ele.requestId || !ele.detectedAnomalies.length}>
            <h3>Anomalies found: ${ele.detectedAnomalies.length}</h3>
            <anomalies-table-sk
              .anomalyList=${ele.detectedAnomalies}
              .isDryRun=${true}></anomalies-table-sk>
          </div>
        `}
  `;

  /** The classname to add to an element if an error has occurred. */
  private static classIfError(hasError: boolean): string {
    return hasError ? 'error' : '';
  }

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
    const ret = [html` <th></th> `];
    if (ClusterLastNPageSk.stepDownAt(ele.state!.direction)) {
      ret.push(html` <th>Low</th> `);
    }
    if (ClusterLastNPageSk.stepUpAt(ele.state!.direction)) {
      ret.push(html` <th>High</th> `);
    }
    if (ClusterLastNPageSk.notBoth(ele.state!.direction)) {
      ret.push(html` <th></th> `);
    }
    return ret;
  };

  private static fullSummary(frame: FrameResponse, summary: ClusterSummary) {
    return {
      frame,
      summary,
    };
  }

  private static low = (ele: ClusterLastNPageSk, reg: RegressionAtCommit) => {
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
            .triage=${reg.regression!.low_status}></triage-status-sk>
        </td>
      `;
    }
    return html` <td class="cluster"></td> `;
  };

  private static high = (ele: ClusterLastNPageSk, reg: RegressionAtCommit) => {
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
            .triage=${reg.regression!.high_status}></triage-status-sk>
        </td>
      `;
    }
    return html` <td class="cluster"></td> `;
  };

  private static filler = (ele: ClusterLastNPageSk) => {
    if (ClusterLastNPageSk.notBoth(ele.state!.direction)) {
      return html` <td></td> `;
    }
    return html``;
  };

  private static tableRows = (ele: ClusterLastNPageSk) =>
    ele.regressions.map(
      (reg) => html`
        <tr>
          <td class="fixed">
            <commit-detail-sk .cid=${reg!.cid}></commit-detail-sk>
          </td>

          ${ClusterLastNPageSk.low(ele, reg!)} ${ClusterLastNPageSk.high(ele, reg!)}
          ${ClusterLastNPageSk.filler(ele)}
        </tr>
      `
    );

  private static configTitle = (ele: ClusterLastNPageSk) => {
    // Original style regression detection is indicated by the empty string for
    // backwards compatibility, so calculate a display value in that case.
    let detection: string = ele.state!.step;
    if (ele.state!.step === '') {
      detection = 'original';
    }
    return html`
      Algo: ${detection}/${ele.state!.algo} - Radius: ${ele.state!.radius} - Sparse:
      ${ele.state!.sparse} - Threshold: ${ele.state!.interesting}
    `;
  };

  private static writeAlertTitle = (ele: ClusterLastNPageSk) => {
    if (ele.state?.id_as_string === '-1') {
      return 'Create Alert';
    }
    return 'Update Alert';
  };

  private static table = (ele: ClusterLastNPageSk) => {
    if (ele.requestId && !ele.regressions.length) {
      return html` No regressions found yet. `;
    }
    return html`
      <table id="regressions-table" @start-triage=${ele.triageStart}>
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

  connectedCallback(): void {
    super.connectedCallback();

    LoggedIn()
      .then((status) => {
        this.user_id = status.email;
      })
      .catch(errorMessage);
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
        this.stateHasChanged = stateReflector(
          () => this.state as unknown as HintableObject,
          (state) => {
            this.state = state as unknown as Alert;
            this._render();
          }
        );
      })
      .catch(errorMessage);
  }

  // Call this anytime something in private state is changed. Will be replaced
  // with the real function once stateReflector has been setup.
  // eslint-disable-next-line @typescript-eslint/no-empty-function
  private stateHasChanged = () => {};

  private alertEdit() {
    this.alertDialog!.showModal();
  }

  private writeAlert() {
    this.writingAlert = true;
    this._render();
    // Post the config.
    fetch('/_/alert/update', {
      method: 'POST',
      body: JSON.stringify(this.state),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then((json: AlertUpdateResponse) => {
        this.state!.id_as_string = json.IDAsString;
        this.writingAlert = false;
        this._render();
      })
      .catch((msg: Error) => {
        this.writingAlert = false;
        this._render();
        // This is likely a permissions issue.
        if (this.user_id) {
          errorMessage('Check with group admin to ensure proper permissions.');
        } else {
          errorMessage(msg);
        }
      });
  }

  private alertClose() {
    this.alertDialog!.close();
  }

  private alertAccept() {
    this.alertDialog!.close();
    this.state = this.alertConfig!.config;
    this.stateHasChanged();
    this._render();
  }

  private triageStart(e: CustomEvent<TriageStatusSkStartTriageEventDetails>) {
    this.dialogState = e.detail;
    this._render();
    this.triageDialog!.showModal();
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

  private catch(msg: Error) {
    this.hasError = true;
    this.requestId = '';
    this.runningStatus = `${msg}`;
    this._render();
    if (msg) {
      errorMessage(msg);
    }
  }

  private async run() {
    this.hasError = false;
    if (this.requestId) {
      errorMessage('There is a pending query already running.');
      return;
    }
    this.domain = this.querySelector<DomainPickerSk>('#range')!.state;

    let endpoint = '/_/dryrun/start';
    let requestBody: any = {
      domain: {
        n: this.domain.num_commits,
        offset: 0,
        end: new Date(this.domain.end * 1000).toISOString(),
      },
      alert: this.state!,
      step: 0,
      total_queries: 1,
    };

    if (this.mode === 'sheriff') {
      const configObj = this.sheriffConfig!.getConfig();
      if (!configObj) {
        errorMessage('Failed to generate Sheriff config.');
        return;
      }
      endpoint = '/_/dryrun/start_sheriff_config';
      requestBody = { config: configObj, domain: requestBody.domain };
    }

    try {
      this.requestId = 'running';
      const finalProg = await startRequest(endpoint, requestBody, {
        pollingIntervalMs: 300,
        onProgressUpdate: (prog: progress.SerializedProgress) => {
          if (prog.results) {
            if (this.mode === 'sheriff') {
              this.detectedAnomalies = prog.results;
            } else {
              this.regressions = prog.results;
            }
          }
          this.runningStatus = prog.messages.map((msg) => `${msg.key}: ${msg.value}`).join('\n');
          this._render();
        },
        onStart: () => (this.runSpinner!.active = true),
        onSettled: () => (this.runSpinner!.active = false),
      });
      if (finalProg.status !== 'Finished') {
        throw new Error(messagesToErrorString(finalProg.messages));
      }
      if (this.mode === 'sheriff') {
        this.detectedAnomalies = finalProg.results || [];
        const improvements = this.detectedAnomalies.filter((a) => a.is_improvement).length;
        const regressions = this.detectedAnomalies.length - improvements;
        this.runningStatus += `\nFinished. Found ${regressions} regressions and ${improvements} improvements.`;
      } else {
        this.regressions = finalProg.results || [];
        this.runningStatus += `\nFinished. Found ${this.regressions.length} regressions.`;
      }
    } catch (error) {
      this.catch(error as Error);
    } finally {
      this.requestId = '';
      this._render();
    }
  }
}

define('cluster-lastn-page-sk', ClusterLastNPageSk);
