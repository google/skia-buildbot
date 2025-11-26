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
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { Anomaly, Issue } from '../json';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { ProjectId } from '../json';
import { upgradeProperty } from '../../../elements-sk/modules/upgradeProperty';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';
import { SpinnerSk } from '../../../elements-sk/modules/spinner-sk/spinner-sk';

import '../../../elements-sk/modules/icons/close-icon-sk';
import '../../../elements-sk/modules/spinner-sk';
import '../window/window';
import { CountMetric, telemetry } from '../telemetry/telemetry';

export class ExistingBugDialogSk extends ElementSk {
  _dialog: HTMLDialogElement | null = null;

  private _spinner: SpinnerSk | null = null;

  _projectId: ProjectId;

  private _form: HTMLFormElement | null = null;

  _anomalies: Anomaly[] = [];

  _traceNames: string[] = [];

  private allProjectIdOptions: ProjectId[] = [];

  private bug_url: string = '';

  private _active: boolean = false;

  private bug_id: number | undefined;

  _associatedBugIds = new Set<number>();

  // maintain a map which maps each bug id associates with its title
  bugIdTitleMap: { [key: number]: string } = {};

  // Host bug url, usually from window.perf.bug_host_url.
  private _bug_host_url: string = window.perf ? window.perf.bug_host_url : '';

  private static allProjectIds = (ele: ExistingBugDialogSk) =>
    ele.allProjectIdOptions.map(
      (p) => html`
        <option
          ?selected=${ele.innerText === p.toString()}
          value=${p.toString()}
          title=${p.toString()}>
          ${p.toString()}
        </option>
      `
    );

  private static template = (ele: ExistingBugDialogSk) => html`
    <dialog id="existing-bug-dialog">
      <h2>Existing Bug</h2>
      <div id="add-to-existing-bug">
        <button id="closeIcon" @click=${ele.closeDialog} type="close">
          <close-icon-sk></close-icon-sk>
        </button>
        <form id="existing-bug-form">
          <label for="existing-bug-dialog-select-project">Project</label>
          <select id="existing-bug-dialog-select-project" @input=${ele.projectIdToggle}>
            ${ExistingBugDialogSk.allProjectIds(ele)}
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
            <spinner-sk id="loading-spinner"></spinner-sk>
          </div>
          <div class="bug-list">${ele.associatedBugListTemplate()}<div>
          <div class="footer">
            <button id="file-button" class="submit" @click=${
              ele.addAnomalyWithExistingBug
            } type="submit">Submit</button>
            <button id="close-button" class="close" @click=${
              ele.closeDialog
            } type="close">Close</button>
          </div>
        </form>
      </div>
    </dialog>
  `;

  constructor() {
    super(ExistingBugDialogSk.template);
    this._projectId = 'chromium';
    this.allProjectIdOptions.push('chromium');
  }

  connectedCallback() {
    super.connectedCallback();
    upgradeProperty(this, '_anomalies');
    upgradeProperty(this, '_associatedBugIds');
    upgradeProperty(this, '_bug_host_url');
    upgradeProperty(this, 'bugIdTitleMap');
    this._render();

    this._spinner = this.querySelector('#loading-spinner');
    this._dialog = this.querySelector('#existing-bug-dialog');
    this._form = this.querySelector('#existing-bug-form');
    this._form!.addEventListener('submit', (e) => {
      e.preventDefault();
      this.addAnomalyWithExistingBug();
    });
    this._form!.addEventListener('close', (e) => {
      e.preventDefault();
      this._associatedBugIds.clear();
    });
  }

  closeDialog(): void {
    this._active = false;
    this._spinner!.active = false;
    this._render();
    this._dialog!.close();
  }

  private projectIdToggle() {}

  /**
   * Read the form for choosing an existing bug number for an anomaly alert.
   * Upon success, we redirect the user in a new tab to the existing bug.
   * Upon failure, we keep the dialog open and show an error toast.
   */
  addAnomalyWithExistingBug(): void {
    this._spinner!.active = true;
    // Disable submit and close button
    this.querySelector('#file-button')!.setAttribute('disabled', 'true');
    this.querySelector('#close-button')!.setAttribute('disabled', 'true');

    this._render();

    // Extract bug_id.
    const bugId = (this.querySelector('#bug_id')! as HTMLInputElement)?.value;
    this.bug_id = +bugId as number;
    let anomalyKeys: string[] = this._anomalies.map((a) => a.id);
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
      trace_names: this._traceNames,
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
        this._spinner!.active = false;
        this.querySelector('#file-button')!.removeAttribute('disabled');
        this.querySelector('#close-button')!.removeAttribute('disabled');
        this.closeDialog();

        // Open the bug page in new window.
        this.bug_url = `https://issues.chromium.org/issues/${this.bug_id as number}`;
        window.open(this.bug_url, '_blank');
        this._render();

        // Update anomalies to reflected the existing Bug Id.
        for (let i = 0; i < this._anomalies.length; i++) {
          this._anomalies[i].bug_id = this.bug_id as number;
        }

        // Let explore-simple-sk and chart-tooltip-sk that anomalies have changed and we need to re-render.
        this.dispatchEvent(
          new CustomEvent('anomaly-changed', {
            bubbles: true,
            composed: true,
            detail: {
              traceNames: this._traceNames,
              anomalies: this._anomalies,
              bugId: this.bug_id,
            },
          })
        );
      })
      .catch(() => {
        this._spinner!.active = false;
        this.querySelector('#file-button')!.removeAttribute('disabled');
        this.querySelector('#close-button')!.removeAttribute('disabled');
        errorMessage(
          'Associate alerts request failed due to an internal server error. Please try again.'
        );
        this._render();
      });
  }

  async fetch_associated_bugs() {
    this._spinner!.active = true;

    await fetch('/_/anomalies/group_report', {
      method: 'POST',
      body: JSON.stringify({
        anomalyIDs: String(this._anomalies.map((a) => a.id)),
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
    this._spinner!.active = false;
    this._render();
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
        this.bugIdTitleMap = {};
        console.info('Issue list length ' + issueList.length);
        issueList.forEach((issue) => {
          const issueid = issue.issueId ? Number(issue.issueId) : 0;
          console.info('Issue id: ' + issueid);
          console.log('issue title: ' + issue.issueState?.title);
          if (this._associatedBugIds.has(issueid)) {
            this.bugIdTitleMap[issueid] = issue.issueState?.title ? issue.issueState!.title : '';
          }
        });
      });
  }

  getAssociatedBugList(anomalies: Anomaly[]) {
    this._associatedBugIds.clear();
    anomalies.forEach((anomaly) => {
      if (anomaly.bug_id && anomaly.bug_id !== 0) {
        this._associatedBugIds.add(anomaly.bug_id);
      }
    });
  }

  private associatedBugListTemplate() {
    if (this._anomalies) {
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

  setAnomalies(anomalies: Anomaly[], traceNames: string[]): void {
    this._anomalies = anomalies;
    this._traceNames = traceNames;
    this._form!.reset();
    this._render();
  }

  open(): void {
    this._render();
    this._dialog!.showModal();
    this._active = true;
  }

  get isActive() {
    return this._active;
  }
}

define('existing-bug-dialog-sk', ExistingBugDialogSk);
