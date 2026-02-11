/**
 * @module modules/regressions-page-sk
 * @description <h2><code>regressions-page-sk</code></h2>
 *
 * This module is a component that displays a list of regressions for a given
 * subscription.
 */
import { html, LitElement } from 'lit';
import { customElement, state, query } from 'lit/decorators.js';
import '../../../elements-sk/modules/spinner-sk';
import '../anomalies-table-sk';
import '../subscription-table-sk';
import { toObject, fromObject } from '../../../infra-sk/modules/query';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import {
  GetSheriffListResponse,
  Anomaly,
  GetAnomaliesResponse,
  Subscription,
  Alert,
} from '../json';
import { AnomaliesTableSk } from '../anomalies-table-sk/anomalies-table-sk';
import { SubscriptionTableSk } from '../subscription-table-sk/subscription-table-sk';
import '@material/web/button/outlined-button.js';
import { HintableObject } from '../../../infra-sk/modules/hintable';
import { errorMessage } from '../errorMessage';
import { CountMetric, telemetry } from '../telemetry/telemetry';
import { equals } from '../../../infra-sk/modules/object';

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
@customElement('regressions-page-sk')
export class RegressionsPageSk extends LitElement {
  private static nextUniqueId = 0;

  private readonly uniqueId = `${RegressionsPageSk.nextUniqueId++}`;

  @state()
  state: State = {
    selectedSubscription: '',
    showTriaged: false,
    showImprovements: false,
    useSkia: false,
  };

  @state()
  subscriptionList: string[] = [];

  @state()
  cpAnomalies: Anomaly[] = [];

  @state()
  private subscription: Subscription | null = null;

  @state()
  private alerts: Alert[] = [];

  // Anomalies table
  @query('#anomaly-table')
  anomaliesTable!: AnomaliesTableSk | null;

  @query('#subscription-table')
  subscriptionTable!: SubscriptionTableSk | null;

  @state()
  showMoreAnomalies = false;

  private anomalyCursor: string | null = null;

  @state()
  private anomaliesLoadingSpinner = false;

  @state()
  private showMoreLoadingSpinner = false;

  createRenderRoot() {
    return this;
  }

  connectedCallback(): void {
    super.connectedCallback();

    window.addEventListener('popstate', this._popstate);

    // Initial fetch from URL
    this._popstate();
    this.state.useSkia = (window as any).perf.fetch_anomalies_from_sql;
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    window.removeEventListener('popstate', this._popstate);
  }

  private _popstate = async () => {
    const defaultState: State = {
      selectedSubscription: '',
      showTriaged: false,
      showImprovements: false,
      useSkia: false,
    };

    const delta = toObject(
      window.location.search.slice(1),
      defaultState as unknown as HintableObject
    );
    const newState = { ...defaultState, ...delta } as unknown as State;

    if (newState.selectedSubscription) {
      localStorage.setItem(LAST_SELECTED_SHERIFF_KEY, newState.selectedSubscription);
    }

    // Ensure selectedSubscription is set
    newState.selectedSubscription =
      newState.selectedSubscription || localStorage.getItem(LAST_SELECTED_SHERIFF_KEY) || '';

    if (!equals(this.state as unknown as HintableObject, newState as unknown as HintableObject)) {
      this.state = newState;
    }

    await this.init();
    if (this.state.selectedSubscription !== '') {
      this.cpAnomalies = [];
      await this.fetchRegressions();
    }
  };

  updated(changedProperties: Map<string, any>) {
    if (changedProperties.has('cpAnomalies') || changedProperties.has('state')) {
      this.updatePageTitle();
    }
  }

  private stateHasChanged() {
    const query = fromObject(this.state as unknown as HintableObject);
    const url = window.location.origin + window.location.pathname + '?' + query;
    if (url !== window.location.href) {
      window.history.pushState(null, '', url);
    }
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

    // This is used only when fetching regressions from SQL, fetching from
    // chromeperf does not utilize this param.
    if (this.state.useSkia && this.cpAnomalies.length > 0) {
      queryMap.set('pagination_offset', this.cpAnomalies.length);
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
    try {
      const response = await fetch(url, {
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      });
      const json: GetAnomaliesResponse = await jsonOrThrow(response);

      if (json.subscription) {
        this.subscription = json.subscription;
        this.alerts = json.alerts || [];
      }
      const regs: Anomaly[] = json.anomaly_list || [];
      this.showMoreAnomalies = !!json.anomaly_cursor;

      this.cpAnomalies = [...this.cpAnomalies, ...regs];
      this.anomalyCursor = json.anomaly_cursor;

      this.updatePageTitle();
    } catch (msg) {
      telemetry.increaseCounter(CountMetric.DataFetchFailure, {
        page: 'regressions',
        endpoint: '/_/anomalies/anomaly_list',
      });
      errorMessage(msg as any);
      this.anomaliesLoadingSpinner = false;
      this.showMoreLoadingSpinner = false;
    } finally {
      this.anomaliesLoadingSpinner = false;
      this.showMoreLoadingSpinner = false;
    }
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
    // Don't reset everything here, rely on fetchRegressions logic or specific filter changes
  }

  render() {
    return html`
      <label for="filter-${this.uniqueId}">Sheriff:</label>
      <select
        id="filter-${this.uniqueId}"
        @input=${(e: InputEvent) => this.filterChange((e.target as HTMLInputElement).value)}>
        <option disabled ?selected=${!this.state.selectedSubscription} value>
          -- select an option --
        </option>
        ${this.subscriptionList.map(
          (s) => html`
            <option ?selected=${this.state.selectedSubscription === s} value=${s} title=${s}>
              ${s}
            </option>
          `
        )}
      </select>
      <spinner-sk id="upper-spin" ?active=${this.anomaliesLoadingSpinner}></spinner-sk>
      <button
        id="btnTriaged"
        @click=${() => this.triagedChange()}
        ?disabled=${!this.state.selectedSubscription}>
        ${this.state.showTriaged ? 'Hide Triaged' : 'Show Triaged'}
      </button>
      <button
        id="btnImprovements"
        @click=${() => this.improvementChange()}
        ?disabled=${!this.state.selectedSubscription}>
        ${this.state.showImprovements ? 'Hide Improvements' : 'Show Improvements'}
      </button>
      <subscription-table-sk
        id="subscription-table"
        .subscription=${this.subscription}
        .alerts=${this.alerts}></subscription-table-sk>
      <anomalies-table-sk
        id="anomaly-table"
        .anomalyList=${this.cpAnomalies}
        .loading=${this.anomaliesLoadingSpinner}></anomalies-table-sk>
      <div id="showmore" ?hidden=${!this.showMoreAnomalies}>
        <button id="showMoreAnomalies" @click=${() => this.onShowMore()}>
          <div>Show More</div>
        </button>
        <spinner-sk ?active=${this.showMoreLoadingSpinner}></spinner-sk>
      </div>
    `;
  }

  async onShowMore() {
    this.anomaliesLoadingSpinner = false;
    this.showMoreLoadingSpinner = true;
    await this.fetchRegressions();
  }

  async improvementChange(): Promise<void> {
    this.state = { ...this.state, showImprovements: !this.state.showImprovements };
    this.cpAnomalies = [];
    this.stateHasChanged();
    // RESET for fresh fetch
    this.cpAnomalies = [];
    this.anomalyCursor = null;
    await this.fetchRegressions();
  }

  async triagedChange(): Promise<void> {
    this.state = { ...this.state, showTriaged: !this.state.showTriaged };
    this.cpAnomalies = [];
    this.stateHasChanged();
    // RESET for fresh fetch
    this.cpAnomalies = [];
    this.anomalyCursor = null;
    await this.fetchRegressions();
  }

  async filterChange(sub: string): Promise<void> {
    localStorage.setItem(LAST_SELECTED_SHERIFF_KEY, sub);
    this.state = { ...this.state, selectedSubscription: sub };
    this.cpAnomalies = [];
    this.anomalyCursor = null;
    this.updatePageTitle();
    this.showMoreAnomalies = false;
    this.stateHasChanged();
    await this.fetchRegressions();
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
