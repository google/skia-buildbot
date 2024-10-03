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
import { Anomaly } from '../json';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { ProjectId } from '../json';
import { upgradeProperty } from '../../../elements-sk/modules/upgradeProperty';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';
import { SpinnerSk } from '../../../elements-sk/modules/spinner-sk/spinner-sk';

import '../../../elements-sk/modules/icons/close-icon-sk';
import '../../../elements-sk/modules/spinner-sk';
import { ToastSk } from '../../../elements-sk/modules/toast-sk/toast-sk';

export class ExistingBugDialogSk extends ElementSk {
  private _dialog: HTMLDialogElement | null = null;

  private _spinner: SpinnerSk | null = null;

  private _projectId: ProjectId;

  private _form: HTMLFormElement | null = null;

  private _anomalies: Anomaly[] = [];

  private _traceNames: string[] = [];

  private allProjectIdOptions: ProjectId[] = [];

  private _toast: ToastSk | null = null;

  private bug_url: string = '';

  private _active: boolean = false;

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
      <header id="add-to-existing-bug">
        <button id="closeIcon" @click=${ele.closeDialog}>
          <close-icon-sk></close-icon-sk>
        </button>
        <form id="existing-bug-form">
          <select id="existing-bug-dialog-select-project" @input=${ele.projectIdToggle}>
            ${ExistingBugDialogSk.allProjectIds(ele)}
          </select>
            <input
              id="bug_id"
              type="text"
              name="bug_id"
              required placeholder="Bug ID"
              pattern="[0-9]+">
              <br><br>
          </div>
          <div class="footer">
            <spinner-sk id="dialog-spinner"></spinner-sk>
            <button id="file-button" type="submit">Submit</button>
            <button id="close-button" @click=${ele.closeDialog} type="button">Close</button>
          </div>
        </form>
      </header>
    </dialog>
        <toast-sk id="bug-url-toast" duration=0>
      Existing Bug update: <a href=${ele.bug_url} target=_blank>${ele.bug_url}</a>
      <button id="hide-toast" class="action" @click=${ele.closeToast}>Close</button>
    </toast-sk>
  `;

  constructor() {
    super(ExistingBugDialogSk.template);
    this._projectId = 'chromium';

    this.allProjectIdOptions.push('chromium');
    this.allProjectIdOptions.push('angleproject');
    this.allProjectIdOptions.push('aomedia');
    this.allProjectIdOptions.push('apvi');
    this.allProjectIdOptions.push('boringssl');
    this.allProjectIdOptions.push('chromedriver');
    this.allProjectIdOptions.push('crashpad');
    this.allProjectIdOptions.push('dawn');
    this.allProjectIdOptions.push('gerrit');
    this.allProjectIdOptions.push('gn');
    this.allProjectIdOptions.push('google-breakpad');
    this.allProjectIdOptions.push('gvp');
    this.allProjectIdOptions.push('libyuv');
    this.allProjectIdOptions.push('linux-syscall-support');
    this.allProjectIdOptions.push('monorail');
    this.allProjectIdOptions.push('nativeclient');
    this.allProjectIdOptions.push('openscreen');
    this.allProjectIdOptions.push('oss-fuzz');
    this.allProjectIdOptions.push('pdfium');
    this.allProjectIdOptions.push('pigweed');
    this.allProjectIdOptions.push('project-zero');
    this.allProjectIdOptions.push('skia');
    this.allProjectIdOptions.push('swiftshader');
    this.allProjectIdOptions.push('tint');
    this.allProjectIdOptions.push('v8');
    this.allProjectIdOptions.push('webm');
    this.allProjectIdOptions.push('webp');
    this.allProjectIdOptions.push('webports');
    this.allProjectIdOptions.push('webrtc');
  }

  connectedCallback() {
    super.connectedCallback();
    upgradeProperty(this, '_anomalies');
    this._render();

    this._toast = this.querySelector('#bug-url-toast');
    this._dialog = this.querySelector('#existing-bug-dialog');
    this._spinner = this.querySelector('#dialog-spinner');
    this._form = this.querySelector('#existing-bug-form');
    this._form!.addEventListener('submit', (e) => {
      e.preventDefault();
      this.addAnomalyWithExistingBug();
    });
  }

  private closeDialog(): void {
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
    document.getElementById('file-button')!.setAttribute('disabled', 'true');
    document.getElementById('close-button')!.setAttribute('disabled', 'true');

    this._render();

    // Extract bug_id.
    const bugId = document.getElementById('bug_id')! as HTMLInputElement;

    // Extract project_id.
    const projectId = document.getElementById(
      'existing-bug-dialog-select-project'
    )! as HTMLInputElement;

    const alertKeys: number[] = this._anomalies.map((a) => a.id);
    const requestBody = {
      bug_id: bugId?.value,
      project_id: projectId?.value,
      keys: alertKeys,
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
      .then((json) => {
        this._spinner!.active = false;
        document.getElementById('file-button')!.removeAttribute('disabled');
        document.getElementById('close-button')!.removeAttribute('disabled');
        this.closeDialog();

        // Open the bug page in new window.
        this.bug_url = `https://issues.chromium.org/issues/${json.bug_id}`;
        window.open(this.bug_url, '_blank');
        this._render();
        this._toast!.show();

        // Update anomalies to reflected the existing Bug Id.
        for (let i = 0; i < this._anomalies.length; i++) {
          this._anomalies[i].bug_id = json.bug_id;
        }

        // Let explore-simple-sk and chart-tooltip-sk that anomalies have changed and we need to re-render.
        this.dispatchEvent(
          new CustomEvent('anomaly-changed', {
            bubbles: true,
          })
        );
      })
      .catch((msg: any) => {
        this._spinner!.active = false;
        document.getElementById('file-button')!.removeAttribute('disabled');
        document.getElementById('close-button')!.removeAttribute('disabled');
        errorMessage(msg);
        this._render();
      });
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

  private closeToast(): void {
    this._toast!.hide();
  }

  get isActive() {
    return this._active;
  }
}

define('existing-bug-dialog-sk', ExistingBugDialogSk);
