/**
 * @module modules/regressions-page-sk
 * @description <h2><code>regressions-page-sk</code></h2>
 *
 * This module is a component that displays a list of regressions for a given
 * subscription.
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import '../../../elements-sk/modules/spinner-sk';
import '../anomalies-table-sk';
import '../subscription-table-sk';
import { stateReflector } from '../../../infra-sk/modules/stateReflector';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { Regression, GetSheriffListResponse, Anomaly, GetAnomaliesResponse } from '../json';
import { AnomaliesTableSk } from '../anomalies-table-sk/anomalies-table-sk';
import { SubscriptionTableSk } from '../subscription-table-sk/subscription-table-sk';
import '@material/web/button/outlined-button.js';
import { HintableObject } from '../../../infra-sk/modules/hintable';
import { errorMessage } from '../errorMessage';
import { CountMetric, telemetry } from '../telemetry/telemetry';

// State is the local UI state of regressions-page-sk
interface State {
  selectedSubscription: string;
  showTriaged: boolean;
  showImprovements: boolean;
  useSkia: boolean;
}

const SHERIFF_LIST_ENDPOINT_LEGACY = '/_/anomalies/sheriff_list';
const ANOMALY_LIST_ENDPOINT_LEGACY = '/_/anomalies/anomaly_list';

const SHERIFF_LIST_ENDPOINT = '/_/anomalies/sheriff_list_skia';
const ANOMALY_LIST_ENDPOINT = '/_/anomalies/anomaly_list_skia';

const LAST_SELECTED_SHERIFF_KEY = 'perf-last-selected-sheriff';

/**
 * RegressionsPageSk is a component that displays a list of regressions
 * for a given subscription.
 */
export class RegressionsPageSk extends ElementSk {
  private static nextUniqueId = 0;

  private readonly uniqueId = `${RegressionsPageSk.nextUniqueId++}`;

  // This is a test comment.
  state: State = {
    selectedSubscription: '',
    showTriaged: false,
    showImprovements: false,
    useSkia: false,
  };

  subscriptionList: string[] = [];

  cpAnomalies: Anomaly[] = [];

  regressions: Regression[] = [];

  filter: HTMLSelectElement | null = null;

  private stateHasChanged = () => {};

  // Anomalies table
  anomaliesTable: AnomaliesTableSk | null = null;

  subscriptionTable: SubscriptionTableSk | null = null;

  btnTriaged: HTMLButtonElement | null = null;

  btnImprovement: HTMLButtonElement | null = null;

  showMoreAnomalies: boolean | null = null;

  anomalyCursor: string | null = null;

  private anomaliesLoadingSpinner = false;

  private showMoreLoadingSpinner = false;

  constructor() {
    super(RegressionsPageSk.template);
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();

    this.btnTriaged = document.getElementById('btnTriaged') as HTMLButtonElement;
    this.btnTriaged!.disabled = true;
    this.btnImprovement = document.getElementById('btnImprovements') as HTMLButtonElement;
    this.btnImprovement!.disabled = true;

    // Set up the state reflector to update the selected subscription
    // in the url as well as the sheriff dropdown.
    this.stateHasChanged = stateReflector(
      /* getState */ () => this.state as unknown as HintableObject,
      /* setState */ async (newState) => {
        const typedNewState = newState as unknown as State;
        if (typedNewState.selectedSubscription) {
          localStorage.setItem(LAST_SELECTED_SHERIFF_KEY, typedNewState.selectedSubscription);
        }
        // Merge the new state from the URL. Properties not in the URL
        // will retain their current values.
        this.state = { ...this.state, ...typedNewState };

        // Ensure selectedSubscription is set, prioritizing URL, then localStorage, then empty.
        this.state.selectedSubscription =
          typedNewState.selectedSubscription ||
          localStorage.getItem(LAST_SELECTED_SHERIFF_KEY) ||
          '';
        await this.init();
        if (this.state.selectedSubscription !== '') {
          this.btnTriaged!.disabled = false;
          this.btnImprovement!.disabled = false;
          this.cpAnomalies = [];
          await this.fetchRegressions();
          this._render();
        }
      }
    );
    this.anomaliesTable = this.querySelector('#anomaly-table') as AnomaliesTableSk;
    this.subscriptionTable = this.querySelector('#subscription-table') as SubscriptionTableSk;

    const showMoreClick = this.querySelector('#showMoreAnomalies') as HTMLElement;
    showMoreClick!.onclick = () => {
      this.anomaliesLoadingSpinner = false;
      this.showMoreLoadingSpinner = true;
      this._render();
    };
  }

  async fetchRegressions(): Promise<void> {
    const queryMap = new Map();
    const s = encodeURIComponent(this.state.selectedSubscription);
    if (s !== '') {
      queryMap.set('sheriff', s);
    }
    if (this.state.showTriaged === true) {
      queryMap.set('triaged', this.state.showTriaged);
    }
    if (this.state.showImprovements === true) {
      queryMap.set('improvements', this.state.showImprovements);
    }
    if (this.anomalyCursor) {
      queryMap.set('anomaly_cursor', this.anomalyCursor);
    }
    const queryPairs: string[] = [];
    let queryStr = '';
    if (queryMap.size > 0) {
      for (const [key, value] of queryMap.entries()) {
        queryPairs.push(`${key}=${value}`);
      }
      queryStr = '?' + queryPairs.join('&');
    }

    let url = '';
    if (this.state.useSkia) {
      url = ANOMALY_LIST_ENDPOINT + queryStr;
    } else {
      url = ANOMALY_LIST_ENDPOINT_LEGACY + queryStr;
    }

    this.anomaliesLoadingSpinner = true;
    this._render();
    await fetch(url, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then(async (response) => {
        const json: GetAnomaliesResponse = response;
        if (json.subscription) {
          this.subscriptionTable!.load(json.subscription, json.alerts!);
        }
        const regs: Anomaly[] = json.anomaly_list || [];
        if (json.anomaly_cursor) {
          this.showMoreAnomalies = true;
        } else {
          this.showMoreAnomalies = false;
        }
        this.cpAnomalies = this.cpAnomalies.concat([...regs]);
        this.anomalyCursor = json.anomaly_cursor;
        await this.anomaliesTable!.populateTable(this.cpAnomalies);
        this.updatePageTitle();
      })
      .catch((msg) => {
        telemetry.increaseCounter(CountMetric.DataFetchFailure, {
          page: 'regressions',
          endpoint: '/_/anomalies/anomaly_list',
        });
        errorMessage(msg);
        this.anomaliesLoadingSpinner = false;
        this.showMoreLoadingSpinner = false;
        this._render();
      });
    this.anomaliesLoadingSpinner = false;
    this.showMoreLoadingSpinner = false;
    this._render();
  }

  private async init() {
    const url = this.state.useSkia ? SHERIFF_LIST_ENDPOINT : SHERIFF_LIST_ENDPOINT_LEGACY;
    const response = await fetch(url, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
    });

    const json: GetSheriffListResponse = await jsonOrThrow(response);
    const subscriptions: string[] = json.sheriff_list || [];
    const sortedSubscriptions: string[] = subscriptions.sort((a, b) =>
      a.toLowerCase().localeCompare(b.toLowerCase())
    );

    this.subscriptionList = [...sortedSubscriptions];
    this.regressions = [];
    this.cpAnomalies = [];
    this.updatePageTitle();
    this.showMoreAnomalies = false;
    this.anomaliesLoadingSpinner = false;
    this.stateHasChanged();
    this._render();
  }

  private static template = (ele: RegressionsPageSk) => html`
    <label for="filter-${ele.uniqueId}">Sheriff:</label>
    <select
      id="filter-${ele.uniqueId}"
      @input=${(e: InputEvent) => ele.filterChange((e.target as HTMLInputElement).value)}>
      <option disabled selected value>-- select an option --</option>
      ${RegressionsPageSk.allSubscriptions(ele)}]
    </select>
    <spinner-sk id="upper-spin" ?active=${ele.anomaliesLoadingSpinner}></spinner-sk>
    <button id="btnTriaged" @click=${() => ele.triagedChange()}>Show Triaged</button>
    <button id="btnImprovements" @click=${() => ele.improvementChange()}>Show Improvements</button>
    <subscription-table-sk id="subscription-table"></subscription-table-sk>
    <anomalies-table-sk id="anomaly-table"></anomalies-table-sk>
    <div id="showmore" ?hidden=${!ele.showMoreAnomalies}>
      <button id="showMoreAnomalies" @click=${() => ele.fetchRegressions()}>
        <div>Show More</div>
      </button>
      <spinner-sk ?active=${ele.showMoreLoadingSpinner}></spinner-sk>
    </div>
    ${ele.regressions.length > 0
      ? html` <div id="regressions_container">${ele.getRegTemplate(ele.regressions)}</div>`
      : null}
  `;

  async improvementChange(): Promise<void> {
    this.state.showImprovements = !this.state.showImprovements;
    if (this.state.showImprovements) {
      this.btnImprovement!.textContent = 'Hide Improvements';
    } else {
      this.btnImprovement!.textContent = 'Show Improvements';
    }
    this.stateHasChanged();
    await this.fetchRegressions();
    this._render();
  }

  async triagedChange(): Promise<void> {
    this.state.showTriaged = !this.state.showTriaged;
    if (this.state.showTriaged) {
      this.btnTriaged!.textContent = 'Hide Triaged';
    } else {
      this.btnTriaged!.textContent = 'Show Triaged';
    }
    this.stateHasChanged();
    await this.fetchRegressions();
    this._render();
  }

  async filterChange(sub: string): Promise<void> {
    localStorage.setItem(LAST_SELECTED_SHERIFF_KEY, sub);
    this.state.selectedSubscription = sub;
    this.btnTriaged!.disabled = false;
    this.btnImprovement!.disabled = false;
    this.cpAnomalies = [];
    this.updatePageTitle();
    this.showMoreAnomalies = false;
    this.anomalyCursor = null;
    this.stateHasChanged();
    await this.fetchRegressions();
    this._render();
  }

  private static allSubscriptions = (ele: RegressionsPageSk) =>
    ele.subscriptionList.map(
      (s) => html`
        <option ?selected=${ele.state.selectedSubscription === s} value=${s} title=${s}>
          ${s}
        </option>
      `
    );

  static isRegressionImprovement = (reg: Regression): boolean => {
    const improvementDirection = reg.frame?.dataframe?.paramset.improvement_direction[0];
    const isDownImprovement =
      improvementDirection === 'down' &&
      reg.cluster_type === 'low' &&
      reg.low?.step_fit?.status === 'Low';
    const isUpImprovement =
      improvementDirection === 'up' &&
      reg.cluster_type === 'high' &&
      reg.high?.step_fit?.status === 'High';

    return isDownImprovement || isUpImprovement;
  };

  private static regRowTemplate = (regInfo: Regression) => html`
    <tr>
      <td>${regInfo.commit_number} - ${regInfo.prev_commit_number}</td>
      <td>${regInfo.frame?.dataframe?.paramset.bot[0]}</td>
      <td>${regInfo.frame?.dataframe?.paramset.benchmark[0]}</td>
      <td>${regInfo.frame?.dataframe?.paramset.test[0]}</td>
      <td class="${this.isRegressionImprovement(regInfo) ? 'green' : 'red'}">
        ${regInfo.frame?.dataframe?.paramset.improvement_direction[0]}
      </td>
      <td class="${this.isRegressionImprovement(regInfo) ? 'green' : 'red'}">
        ${((regInfo.median_after - regInfo.median_before) * 100) / regInfo.median_before}
      </td>
      <td class="${this.isRegressionImprovement(regInfo) ? 'green' : 'red'}">
        ${regInfo.median_after - regInfo.median_before}
      </td>
    </tr>
  `;

  private getRegTemplate(regs: Regression[]) {
    return html` <table class="sortable">
      <tr>
        <th>Revisions</th>
        <th>Bot</th>
        <th>Benchmark</th>
        <th>Test</th>
        <th>Change Direction</th>
        <th>Delta</th>
        <th>Delta Abs</th>
      </tr>
      ${regs.map((regression) => RegressionsPageSk.regRowTemplate(regression))}
    </table>`;
  }

  private updatePageTitle(): void {
    const anomalyCount = this.cpAnomalies.length;
    let title = 'Regressions';
    if (anomalyCount > 0) {
      const triagedText = this.state.showTriaged ? ' total' : ' untriaged';
      title = `Regressions (${anomalyCount}${triagedText})`;
    }
    document.title = title;
  }
}

define('regressions-page-sk', RegressionsPageSk);
