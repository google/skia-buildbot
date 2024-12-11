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
import { stateReflector } from '../../../infra-sk/modules/stateReflector';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { Regression, GetSheriffListResponse, Anomaly, GetAnomaliesResponse } from '../json';
import { AnomaliesTableSk } from '../anomalies-table-sk/anomalies-table-sk';
import '@material/web/button/outlined-button.js';
import { HintableObject } from '../../../infra-sk/modules/hintable';
import { errorMessage } from '../errorMessage';

// State is the local UI state of regressions-page-sk
interface State {
  selectedSubscription: string;
  showTriaged: boolean;
  showImprovements: boolean;
}

const SHERIFF_LIST_ENDPOINT = '/_/anomalies/sheriff_list';
const ANOMALY_LIST_ENDPOINT = '/_/anomalies/anomaly_list';

/**
 * RegressionsPageSk is a component that displays a list of regressions
 * for a given subscription.
 */
export class RegressionsPageSk extends ElementSk {
  state: State;

  subscriptionList: string[] = [];

  cpAnomalies: Anomaly[] = [];

  regressions: Regression[] = [];

  filter: HTMLSelectElement | null = null;

  private stateHasChanged = () => {};

  // Anomalies table
  anomaliesTable: AnomaliesTableSk | null = null;

  btnTriaged: HTMLButtonElement | null = null;

  btnImprovement: HTMLButtonElement | null = null;

  showMoreAnomalies: boolean | null = null;

  anomalyCursor: string | null = null;

  private anomaliesLoadingSpinner = false;

  private showMoreLoadingSpinner = false;

  constructor() {
    super(RegressionsPageSk.template);
    this.state = {
      selectedSubscription: '',
      showTriaged: false,
      showImprovements: false,
    };
    this.anomaliesTable = new AnomaliesTableSk();
    this.init();
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
        this.state = newState as unknown as State;
        if (this.state.selectedSubscription !== '') {
          this.btnTriaged!.disabled = false;
          this.btnImprovement!.disabled = false;
          this.cpAnomalies = [];
          await this.fetchRegressions();
          this._render();
        }
      }
    );
    this.anomaliesTable = document.getElementById('anomaly-table') as AnomaliesTableSk;
    const showMoreClick = document.getElementById('showMoreAnomalies');
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
    const queryPairs = [];
    let queryStr = '';
    if (queryMap.size > 0) {
      for (const [key, value] of queryMap.entries()) {
        queryPairs.push(`${key}=${value}`);
      }
      queryStr = '?' + queryPairs.join('&');
    }

    const url = ANOMALY_LIST_ENDPOINT + queryStr;

    this.anomaliesLoadingSpinner = true;
    this._render();
    await fetch(url, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .catch((msg) => {
        errorMessage(msg);
        this.anomaliesLoadingSpinner = false;
        this.showMoreLoadingSpinner = false;
        this._render();
      })
      .then((response) => {
        const json: GetAnomaliesResponse = response;
        const regs: Anomaly[] = json.anomaly_list || [];
        if (json.anomaly_cursor) {
          this.showMoreAnomalies = true;
        } else {
          this.showMoreAnomalies = false;
        }
        this.cpAnomalies = this.cpAnomalies.concat([...regs]);
        this.anomalyCursor = json.anomaly_cursor;
        this.anomaliesTable!.populateTable(this.cpAnomalies);
      });
    this.anomaliesLoadingSpinner = false;
    this.showMoreLoadingSpinner = false;
    this._render();
  }

  private async init() {
    const response = await fetch(SHERIFF_LIST_ENDPOINT, {
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
    this.showMoreAnomalies = false;
    this.anomaliesLoadingSpinner = false;
    this.stateHasChanged();
    this._render();
  }

  private static template = (ele: RegressionsPageSk) => html`
    <label for="filter">Sheriff:</label>
    <select
      id="filter"
      @input=${(e: InputEvent) => ele.filterChange((e.target as HTMLInputElement).value)}>
      <option disabled selected value>-- select an option --</option>
      ${RegressionsPageSk.allSubscriptions(ele)}]
    </select>
    <spinner-sk id="upper-spin" ?active=${ele.anomaliesLoadingSpinner}></spinner-sk>
    <button id="btnTriaged" @click=${() => ele.triagedChange()}>Show Triaged</button>
    <button id="btnImprovements" @click=${() => ele.improvementChange()}>Show Improvements</button>
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
    this.state.selectedSubscription = sub;
    this.btnTriaged!.disabled = false;
    this.btnImprovement!.disabled = false;
    this.cpAnomalies = [];
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
}

define('regressions-page-sk', RegressionsPageSk);
