/**
 * @module modules/revision-info-sk
 * @description <h2><code>revision-info-sk</code></h2>
 *
 * Displays information regarding the anomalies that were detected around
 * a specific revision.
 */
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { RevisionInfo } from '../json';
import '../../../elements-sk/modules/spinner-sk';
import '../../../elements-sk/modules/checkbox-sk';
import { stateReflector } from '../../../infra-sk/modules/stateReflector';
import { HintableObject } from '../../../infra-sk/modules/hintable';
import { GraphConfig, updateShortcut } from '../common/graph-config';
import { CheckOrRadio } from '../../../elements-sk/modules/checkbox-sk/checkbox-sk';
import { errorMessage } from '../errorMessage';

class State {
  revisionId: number = 0; // The revisionId.
}

export class RevisionInfoSk extends ElementSk {
  constructor() {
    super(RevisionInfoSk.template);
  }

  revisionId: HTMLInputElement | null = null;

  revisionInfos: RevisionInfo[] | null = null;

  private revisionInfoContainer: HTMLDivElement | null = null;

  private showSpinner: boolean = false;

  private selectAll: boolean = false;

  private enableMultiGraph: boolean = false;

  // eslint-disable-next-line @typescript-eslint/no-empty-function
  private stateHasChanged = () => {};

  private state: State = {
    revisionId: 0,
  };

  // toggleSelectAll checks/unchecks all revision ranges on the page
  public async toggleSelectAll() {
    this.selectAll = !this.selectAll;
    this.querySelectorAll<CheckOrRadio>('#selectRevision').forEach((ele) => {
      ele.checked = this.selectAll;
    });

    this.updateMultiGraphStatus();
    this._render();
  }

  // viewMultiGraph redirects to multi graph view for the checked revision ranges
  public async viewMultiGraph(): Promise<void> {
    const selectedRevs: RevisionInfo[] = [];

    this.querySelectorAll<HTMLInputElement>('#selectRevision').forEach((ele, i) => {
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

  // getMultiGraphUrl generates the redirect url for the multigraph
  // view of the checked revision ranges
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
    // Gather the unique anomaly ids from all revisioninfos.
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

    const url =
      `/m/?begin=${begin}&end=${end}` +
      `&shortcut=${newShortcut}` +
      `&totalGraphs=${graphConfigs.length}` +
      `${highlightAnomalies}`;

    return url;
  }

  // getGraphConfigs generates GraphConfig[] object from the checked revision ranges
  // provided as func args.
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

  private static template = (ele: RevisionInfoSk) => html`
    <h3>Revision Information</h3>
    <input id=revision_id type=text alt="Revision Id"></input>
    <button id="getRevisionInfo"  @click=${() =>
      ele.getRevisionInfo()}>Get Revision Information</button>
    <div>
    <button
      id="getRevisionInfo"
      ?disabled=${!ele.enableMultiGraph}
      @click=${() => ele.viewMultiGraph()}
    >
      View Selected Graph(s)
    </button>
    <div>
      <spinner-sk ?active=${ele.showSpinner}></spinner-sk>
    </div>
    <div id="revision_info_container" hidden=true>
      ${ele.getRevInfosTemplate()}
    </div>
    `;

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

  private updateMultiGraphStatus = () => {
    const hasCheckedRevs =
      this.querySelectorAll<HTMLInputElement>('#selectRevision input:checked').length > 0;

    if (!hasCheckedRevs) {
      this.selectAll = false;
    }

    this.enableMultiGraph = hasCheckedRevs;
    this._render();
  };

  // Return the template for an individual row.
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

  connectedCallback(): void {
    super.connectedCallback();
    this._render();
    this.revisionId = this.querySelector('#revision_id');
    this.revisionInfoContainer = this.querySelector('#revision_info_container');

    // Set up the state reflector to update the revision id values
    // in the url as well as the text box.
    this.stateHasChanged = stateReflector(
      /* getState */ () => this.state as unknown as HintableObject,
      /* setState */ async (newState) => {
        this.state = newState as unknown as State;
        if (this.state.revisionId > 0) {
          this.revisionId!.value = this.state.revisionId.toString();
          if (this.state.revisionId > 0) {
            await this.getRevisionInfo();
          }
        }
      }
    );
  }

  async getRevisionInfo(): Promise<void> {
    // Update the UI to reflect the specified revision id and display spinner
    this.showSpinner = true;
    this.revisionInfoContainer!.hidden = true;
    this.state.revisionId = +this.revisionId!.value;
    this.stateHasChanged();
    this._render();

    // Send the request to get the revision info items to display
    const response = await fetch(`/_/revision/?rev=${this.revisionId!.value}`);
    const json = await jsonOrThrow(response);
    this.revisionInfos = json;
    this.revisionInfoContainer!.hidden = false;
    this.showSpinner = false;
    this._render();
  }
}

define('revision-info-sk', RevisionInfoSk);
