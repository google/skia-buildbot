/**
 * @module modules/revision-info-sk
 * @description <h2><code>revision-info-sk</code></h2>
 *
 * Displays information regarding the anomalies that were detected around
 * a specific revision.
 */
import { LitElement, html } from 'lit';
import { customElement, property, state } from 'lit/decorators.js';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { RevisionInfo } from '../json';
import '../../../elements-sk/modules/spinner-sk';
import '../../../elements-sk/modules/checkbox-sk';
import { stateReflector } from '../../../infra-sk/modules/stateReflector';
import { HintableObject } from '../../../infra-sk/modules/hintable';
import { GraphConfig, updateShortcut } from '../common/graph-config';
import { CheckOrRadio } from '../../../elements-sk/modules/checkbox-sk/checkbox-sk';
import { errorMessage } from '../errorMessage';
import '../window/window';

class State {
  revisionId: number = 0; // The revisionId.
}

@customElement('revision-info-sk')
export class RevisionInfoSk extends LitElement {
  @property({ type: Array })
  revisionInfos: RevisionInfo[] | null = null;

  @state()
  private showSpinner = false;

  @state()
  private selectAll = false;

  @state()
  private enableMultiGraph = false;

  @state()
  private state: State = {
    revisionId: 0,
  };

  private stateHasChanged = () => {};

  get revisionId() {
    return this.querySelector<HTMLInputElement>('#revision_id');
  }

  protected createRenderRoot() {
    return this;
  }

  connectedCallback() {
    super.connectedCallback();
    this.stateHasChanged = stateReflector(
      /* getState */ () => this.state as unknown as HintableObject,
      /* setState */ async (newState) => {
        this.state = newState as unknown as State;
        if (this.state.revisionId > 0) {
          if (this.revisionId) {
            this.revisionId.value = this.state.revisionId.toString();
          }
          await this.getRevisionInfo();
        }
      }
    );
  }

  render() {
    return html`
      <h3>Revision Information</h3>
      <input id="revision_id" type="text" alt="Revision Id" .value=${
        this.state.revisionId.toString() || ''
      }></input>
      <button id="getRevisionInfo" @click=${
        this._onGetRevisionInfo
      }>Get Revision Information</button>
      <div>
        <button
          id="viewMultiGraph"
          ?disabled=${!this.enableMultiGraph}
          @click=${this.viewMultiGraph}
        >
          View Selected Graph(s)
        </button>
      </div>
      <div>
        <spinner-sk ?active=${this.showSpinner}></spinner-sk>
      </div>
      <div id="revision_info_container" ?hidden=${!this.revisionInfos}>
        ${this.getRevInfosTemplate()}
      </div>
    `;
  }

  private async _onGetRevisionInfo() {
    if (this.revisionId) {
      this.state.revisionId = +this.revisionId.value;
    }
    this.stateHasChanged();
    await this.getRevisionInfo();
  }

  private getRevInfosTemplate() {
    if (this.revisionInfos === null) {
      return html``;
    }
    return html`
      <table class="sortable">
        <tr>
          <th>
            <checkbox-sk
              ?checked=${this.selectAll}
              @change=${this.toggleSelectAll}
              id="selectAllRevisions">
            </checkbox-sk>
          </th>
          <th>Bug ID</th>
          <th>Revisions</th>
          <th>Master</th>
          <th>Bot</th>
          <th>Benchmark</th>
          <th>Test</th>
        </tr>
        ${this.revisionInfos.map((revInfo) => this.revInfoRowTemplate(revInfo))}
      </table>
    `;
  }

  private revInfoRowTemplate = (revInfo: RevisionInfo) => html`
    <tr>
      <td>
        <checkbox-sk id="selectRevision" @change=${this.updateMultiGraphStatus}> </checkbox-sk>
      </td>
      <td>
        <a target="_blank" href="http://crbug/${revInfo.bug_id}">${revInfo.bug_id}</a>
      </td>
      <td>
        <a target="_blank" href="${revInfo.explore_url}"
          >${revInfo.start_revision} - ${revInfo.end_revision}</a
        >
      </td>
      <td>${revInfo.master}</td>
      <td>${revInfo.bot}</td>
      <td>${revInfo.benchmark}</td>
      <td>${revInfo.test}</td>
    </tr>
  `;

  public async toggleSelectAll() {
    this.selectAll = !this.selectAll;
    this.querySelectorAll<CheckOrRadio>('#selectRevision').forEach((ele) => {
      ele.checked = this.selectAll;
    });

    this.updateMultiGraphStatus();
  }

  private updateMultiGraphStatus = () => {
    const hasCheckedRevs = Array.from(this.querySelectorAll<CheckOrRadio>('#selectRevision')).some(
      (ele) => ele.checked
    );

    if (!hasCheckedRevs) {
      this.selectAll = false;
    }

    this.enableMultiGraph = hasCheckedRevs;
  };

  public async viewMultiGraph(): Promise<void> {
    const selectedRevs: RevisionInfo[] = [];

    this.querySelectorAll<CheckOrRadio>('#selectRevision').forEach((ele, i) => {
      if (this.revisionInfos && this.revisionInfos[i] && ele.checked) {
        selectedRevs.push(this.revisionInfos[i]);
      }
    });

    const url = await this.getMultiGraphUrl(selectedRevs);

    if (url === '') {
      errorMessage(
        `Failed to view graph. Please file a bug at ${(window as any).perf.feedback_url}`
      );
      return;
    }

    window.open(url, '_self');
  }

  async getMultiGraphUrl(revisions: RevisionInfo[]): Promise<string> {
    const graphConfigs: GraphConfig[] = this.getGraphConfigs(revisions);

    const newShortcut = await updateShortcut(graphConfigs);

    if (newShortcut === '') {
      return '';
    }

    const begin: number = revisions.map((rev) => rev.start_time).sort()[0];
    const end: number = revisions
      .map((rev) => rev.end_time)
      .sort()
      .reverse()[0];

    const uniqueAnomalies: Set<string> = new Set();
    let highlightAnomalies = '';
    revisions
      .map((rev) => rev.anomaly_ids)
      .forEach((anomalyArr) => {
        anomalyArr!.forEach((anomalyId) => {
          uniqueAnomalies.add(anomalyId);
        });
      });
    uniqueAnomalies.forEach((anomalyId) => {
      highlightAnomalies += `&highlight_anomalies=${anomalyId}`;
    });

    let url = `/m/?begin=${begin}&end=${end}&shortcut=${newShortcut}${highlightAnomalies}`;
    if (window.perf && window.perf.default_to_manual_plot_mode) {
      url += '&manual_plot_mode=true';
    }

    return url;
  }

  getGraphConfigs(revisions: RevisionInfo[]): GraphConfig[] {
    const graphConfigs: GraphConfig[] = [];

    revisions.forEach((rev) => {
      graphConfigs.push({
        formulas: [],
        queries: [rev.query],
        keys: '',
      });
    });

    return graphConfigs;
  }

  async getRevisionInfo(): Promise<void> {
    this.showSpinner = true;
    if (this.revisionId && this.revisionId.value) {
      this.state.revisionId = +this.revisionId.value;
    }
    const response = await fetch(`/_/revision/?rev=${this.state.revisionId}`);
    const json = await jsonOrThrow(response);
    this.revisionInfos = json;
    this.showSpinner = false;
  }
}
