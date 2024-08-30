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
import { stateReflector } from '../../../infra-sk/modules/stateReflector';
import { HintableObject } from '../../../infra-sk/modules/hintable';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { Regression, Subscription } from '../json';

// State is the local UI state of regressions-page-sk
interface State {
  selectedSubscription: string;
}

/**
 * RegressionsPageSk is a component that displays a list of regressions
 * for a given subscription.
 */
export class RegressionsPageSk extends ElementSk {
  state: State;

  private subscriptionList: Subscription[] = [];

  regressions: Regression[] = [];

  filter: HTMLSelectElement | null = null;

  private stateHasChanged = () => {};

  constructor() {
    super(RegressionsPageSk.template);
    this.state = {
      selectedSubscription: '',
    };

    this.init();
  }

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    // Set up the state reflector to update the selected subscription
    // in the url as well as the sheriff dropdown.
    this.stateHasChanged = stateReflector(
      /* getState */ () => this.state as unknown as HintableObject,
      /* setState */ async (state) => {
        this.state = state as unknown as State;
        if (this.state.selectedSubscription !== '') {
          await this.fetchRegressions();
          this._render();
        }
      }
    );
  }

  private async fetchRegressions(): Promise<void> {
    const s = this.state.selectedSubscription;
    const url = `/_/regressions?sub_name=${s}&limit=${10}&offset=${0}`;
    const response = await fetch(url, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
    });
    const json = await jsonOrThrow(response);
    const regs: Regression[] = json;
    this.regressions = [...regs];
  }

  private async init() {
    const response = await fetch('/_/subscriptions', {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
    });
    const json = await jsonOrThrow(response);
    const subscriptions: Subscription[] = json;

    this.subscriptionList = [...subscriptions];
    this.regressions = [];
    this.stateHasChanged();
    this._render();
  }

  private static template = (ele: RegressionsPageSk) => html`
    <label for="filter">Sheriff:</label>
    <select
      id="filter"
      @input=${(e: InputEvent) =>
        ele.filterChange((e.target as HTMLInputElement).value)}>
      <option disabled selected value>-- select an option --</option>
      ${RegressionsPageSk.allSubscriptions(ele)}]
    </select>
    ${ele.regressions.length > 0
      ? html` <div id="regressions_container">
          ${ele.getRegTemplate(ele.regressions)}
        </div>`
      : null}
  `;

  async filterChange(sub: string): Promise<void> {
    this.state.selectedSubscription = sub;
    this.stateHasChanged();
    await this.fetchRegressions();
    this._render();
  }

  private static allSubscriptions = (ele: RegressionsPageSk) =>
    ele.subscriptionList.map(
      (s) => html`
        <option
          ?selected=${ele.state.selectedSubscription === s.name}
          value=${s.name}
          title=${s.name}>
          ${s.name}
        </option>
      `
    );

  static isRegressionImprovement = (reg: Regression): boolean => {
    const improvementDirection =
      reg.frame?.dataframe?.paramset.improvement_direction[0];
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
        ${((regInfo.median_after - regInfo.median_before) * 100) /
        regInfo.median_before}
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
