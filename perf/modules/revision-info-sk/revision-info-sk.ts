/**
 * @module modules/revision-info-sk
 * @description <h2><code>revision-info-sk</code></h2>
 *
 * Displays information regarding the anomalies that were detected around
 * a specific revision.
 */
import { html } from 'lit-html';
import { define } from '../../../elements-sk/modules/define';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { RevisionInfo } from '../json';
import '../../../elements-sk/modules/spinner-sk';
import { stateReflector } from '../../../infra-sk/modules/stateReflector';
import { HintableObject } from '../../../infra-sk/modules/hintable';

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

  // eslint-disable-next-line @typescript-eslint/no-empty-function
  private stateHasChanged = () => {};

  private state: State = {
    revisionId: 0,
  };

  private static template = (ele: RevisionInfoSk) => html`
    <h3>Revision Information</h3>
    <input id=revision_id type=text alt="Revision Id"></input>
    <button id="getRevisionInfo"  @click=${() =>
      ele.getRevisionInfo()}>Get Revision Information</button>
    <div>
      <spinner-sk ?active=${ele.showSpinner}></spinner-sk>
    </div>
    <div id="revision_info_container" hidden=true>
      ${ele.getRevInfosTemplate()}
    </div>
    `;

  private getRevInfosTemplate() {
    if (this.revisionInfos == null) {
      return html``;
    }
    return html`
      <table class="sortable">
        <tr>
          <th>Bug ID</th>
          <th>Revisions</th>
          <th>Master</th>
          <th>Bot</th>
          <th>Benchmark</th>
          <th>Test</th>
        </tr>
        ${this.revisionInfos.map((revInfo) =>
          RevisionInfoSk.revInfoRowTemplate(revInfo)
        )}
      </table>
    `;
  }

  // Return the template for an individual row.
  private static revInfoRowTemplate = (revInfo: RevisionInfo) => html`
    <tr>
      <td>
        <a target="_blank" href="http://crbug/${revInfo.bug_id}"
          >${revInfo.bug_id}</a
        >
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
