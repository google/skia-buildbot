/**
 * @module modules/existing-bug-dialog-sk
 * @description <h2><code>existing-bug-dialog-sk</code></h2>
 *
 * Dialog to show when user wants to edit a bug to associate with the alert.
 *
 * Takes the following inputs:
 * Request parameters:
 *   bug_id: Bug ID number, as a string (when submitting the form).
 *   project_id: Monorail project ID (when submitting the form).
 *   keys: Comma-separated alert keys in urlsafe format (when submitting the form).
 *   confirm: If non-empty, associate alerts with a bug ID even if
 *       it appears that the alerts already associated with that bug
 *       have a non-overlapping revision range.
 *
 * Once a validated user submits this dialog, there'll be an attempt to submit a
 * bug id and a project id. If succesful, user is re-directed to the another page and close the dialog. If unsuccesful,
 * an error message toast will appear.
 *
 */

import '../../../elements-sk/modules/select-sk';
import { html, LitElement, PropertyValues } from 'lit';
import { customElement, property, state, query } from 'lit/decorators.js';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { Anomaly, Issue } from '../json';
import { ProjectId } from '../json';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';
import { SpinnerSk } from '../../../elements-sk/modules/spinner-sk/spinner-sk';

import '../../../elements-sk/modules/icons/close-icon-sk';
import '../../../elements-sk/modules/spinner-sk';
import '../window/window';
import { CountMetric, telemetry } from '../telemetry/telemetry';

@customElement('existing-bug-dialog-sk')
export class ExistingBugDialogSk extends LitElement {
  @query('#existing-bug-dialog')
  private _dialog!: HTMLDialogElement;

  @query('#loading-spinner')
  private _spinner!: SpinnerSk;

  @query('#existing-bug-form')
  private _form!: HTMLFormElement;

  @query('#bug_id')
  private _bugIdInput!: HTMLInputElement;

  @state()
  private _projectId: ProjectId = 'chromium';

  @property({ attribute: false })
  anomalies: Anomaly[] = [];

  @property({ attribute: false })
  traceNames: string[] = [];

  setAnomalies(anomalies: Anomaly[], traceNames: string[]): void {
    this.anomalies = anomalies;
    this.traceNames = traceNames;
  }

  updated(changedProperties: PropertyValues) {
    if (changedProperties.has('anomalies') || changedProperties.has('traceNames')) {
      if (this._form) {
        this._form.reset();
      }
    }
  }

  private allProjectIdOptions: ProjectId[] = ['chromium'];

  private bug_url: string = '';

  @state()
  private _active: boolean = false;

  private bug_id: number | undefined;

  @state()
  _associatedBugIds = new Set<number>();

  // maintain a map which maps each bug id associates with its title
  @state()
  bugIdTitleMap: { [key: number]: string } = {};

  // Host bug url, usually from window.perf.bug_host_url.
  @state()
  private _bug_host_url: string = window.perf ? window.perf.bug_host_url : '';

  @state()
  private _busy: boolean = false;

  createRenderRoot() {
    // Render to the component itself to preserve existing styling mechanisms.
    return this;
  }

  private static allProjectIds = (ele: ExistingBugDialogSk) =>
    ele.allProjectIdOptions.map(
      (p) => html` <option ?selected=${ele._projectId === p} value=${p} title=${p}>${p}</option> `
    );

  render() {
    return html`
    <dialog id="existing-bug-dialog">
      <h2>Existing Bug</h2>
      <div id="add-to-existing-bug">
        <button id="closeIcon" @click=${this.closeDialog} type="close">
          <close-icon-sk></close-icon-sk>
        </button>
        <form id="existing-bug-form" @submit=${this._onSubmit} @close=${this._onClose}>
          <label for="existing-bug-dialog-select-project">Project</label>
          <select id="existing-bug-dialog-select-project" @input=${this.projectIdToggle}>
            ${ExistingBugDialogSk.allProjectIds(this)}
          </select>
          <label for="bug_id">Bug ID</label>
          <input type="text"
            id="bug_id"
            placeholder="123456"
            pattern="[0-9]{5,9}"
            title="Bug ID must be a number, between 5 and 9 digits."
            required autocomplete="off"></input>
          <br></br>
          <div>
            <spinner-sk id="loading-spinner" ?active=${this._busy}></spinner-sk>
          </div>
          <div class="bug-list">${this.associatedBugListTemplate()}<div>
          <div class="footer">
            <button id="file-button" class="submit" @click=${
              this.addAnomalyWithExistingBug
            } type="submit" ?disabled=${this._busy}>Submit</button>
            <button id="close-button" class="close" @click=${
              this.closeDialog
            } type="close" ?disabled=${this._busy}>Close</button>
          </div>
        </form>
      </div>
    </dialog>
  `;
  }

  private _onSubmit(e: Event) {
    e.preventDefault();
    this.addAnomalyWithExistingBug();
  }

  private _onClose(e: Event) {
    e.preventDefault();
    this._associatedBugIds.clear();
  }

  closeDialog(): void {
    this._active = false;
    this._busy = false;
    if (this._dialog) {
      this._dialog.close();
    }
  }

  private projectIdToggle(e: Event) {
    const select = e.target as HTMLSelectElement;
    this._projectId = select.value as ProjectId;
  }

  /**
   * Read the form for choosing an existing bug number for an anomaly alert.
   * Upon success, we redirect the user in a new tab to the existing bug.
   * Upon failure, we keep the dialog open and show an error toast.
   */

  async addAnomalyWithExistingBug() {
    this._busy = true;

    // Extract bug_id.
    const bugId = this._bugIdInput?.value;
    this.bug_id = +bugId as number;
    let anomalyKeys: string[] = this.anomalies.map((a) => a.id);
    if (anomalyKeys.length === 0) {
      telemetry.increaseCounter(CountMetric.ExistingBugDialogSkBugIdUsedAsAnomalyKey, {
        module: 'existing-bug-dialog-sk',
        function: 'addAnomalyWithExistingBug',
      });
      anomalyKeys = [this.bug_id.toString()];
    }

    const requestBody = {
      bug_id: this.bug_id,
      keys: anomalyKeys,
      trace_names: this.traceNames,
    };

    fetch('/_/triage/associate_alerts', {
      method: 'POST',
      body: JSON.stringify(requestBody),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then(() => {
        this._busy = false;
        this.closeDialog();

        // Open the bug page in new window.
        this.bug_url = `https://issues.chromium.org/issues/${this.bug_id as number}`;
        window.open(this.bug_url, '_blank');

        // Update anomalies to reflected the existing Bug Id.
        const newAnomalies = this.anomalies.map((a) => ({ ...a, bug_id: this.bug_id as number }));
        this.anomalies = newAnomalies; // Triggers update if we were open

        // Let explore-simple-sk and chart-tooltip-sk that anomalies have changed and we need to re-render.
        this.dispatchEvent(
          new CustomEvent('anomaly-changed', {
            bubbles: true,
            composed: true,
            detail: {
              traceNames: this.traceNames,
              anomalies: this.anomalies,
              bugId: this.bug_id,
            },
          })
        );
      })
      .catch(() => {
        this._busy = false;
        errorMessage(
          'Associate alerts request failed due to an internal server error. Please try again.'
        );
      });
  }

  async fetch_associated_bugs() {
    this._busy = true;

    await fetch('/_/anomalies/group_report', {
      method: 'POST',
      body: JSON.stringify({
        anomalyIDs: String(this.anomalies.map((a) => a.id)),
      }),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then(async (json) => {
        // Make the .then callback async
        // Prefer anomaly_list over making another call to get SID.
        if (json.anomaly_list) {
          const anomalies: Anomaly[] = json.anomaly_list || [];
          this.getAssociatedBugList(anomalies);
        } else {
          const sid: string = json.sid;
          telemetry.increaseCounter(CountMetric.SIDRequiringActionTaken, {
            module: 'existing-bug-dialog-sk',
            function: 'fetch_associated_bugs',
          });
          await this.fetch_associated_bugs_withSid(sid);
        }
        if (this._associatedBugIds.size !== 0) {
          await this.fetch_bug_titles(); // Await the fetch_bug_titles() call
        }
      });
    this._busy = false;
  }

  private async fetch_associated_bugs_withSid(sid: string) {
    await fetch('/_/anomalies/group_report', {
      method: 'POST',
      body: JSON.stringify({
        StateId: sid,
      }),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then((json) => {
        const anomalies: Anomaly[] = json.anomaly_list || [];
        this.getAssociatedBugList(anomalies);
      });
  }

  async fetch_bug_titles() {
    const bugIds = Array.from(this._associatedBugIds);

    await fetch('/_/triage/list_issues', {
      method: 'POST',
      body: JSON.stringify({
        IssueIds: bugIds,
      }),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then((json) => {
        const issueList: Issue[] = json.issues;
        const newMap: { [key: number]: string } = {};
        console.info('Issue list length ' + issueList.length);
        issueList.forEach((issue) => {
          const issueid = issue.issueId ? Number(issue.issueId) : 0;
          console.info('Issue id: ' + issueid);
          console.log('issue title: ' + issue.issueState?.title);
          if (this._associatedBugIds.has(issueid)) {
            newMap[issueid] = issue.issueState?.title ? issue.issueState!.title : '';
          }
        });
        this.bugIdTitleMap = newMap;
      });
  }

  getAssociatedBugList(anomalies: Anomaly[]) {
    const newSet = new Set<number>();
    anomalies.forEach((anomaly) => {
      if (anomaly.bug_id && anomaly.bug_id !== 0) {
        newSet.add(anomaly.bug_id);
      }
    });
    this._associatedBugIds = newSet;
  }

  private associatedBugListTemplate() {
    if (this.anomalies) {
      if (this._associatedBugIds.size === 0) {
        return html``;
      }

      return html`
        <h4>Associated bugs in the same Anomaly group</h4>
        <ul id="associated-bugs-table">
          ${Array.from(this._associatedBugIds).map((bugId) => {
            return html` <li>
              <a href="${`${this._bug_host_url}/${bugId}`}" target="_blank">${bugId}</a>
              <span id="bug-title">${this.bugIdTitleMap[bugId]}</span>
            </li>`;
          })}
        </ul>
      `;
    }
    return html``;
  }

  open(): void {
    if (this._dialog) {
      this._dialog.showModal();
    }
    this._active = true;
  }

  get isActive() {
    return this._active;
  }
}
