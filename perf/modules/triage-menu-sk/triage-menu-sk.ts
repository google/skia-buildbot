/**
 * @module modules/triage-menu-sk
 * @description <h2><code>triage-menu-sk</code></h2>
 *
 * Triage Menu provides functionality to triage anomalies in bulk. These are the provided features:
 * - New Bug: Creates a bug.
 * - Existing Bug: Adds anomalies to an existing bug.
 * - Ignore: Marks anomalies as Ignored
 *
 * @evt anomaly-changed Sent whenever an anomaly has been modified and needs to be re-rendered.
 */
import { html, TemplateResult } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';
import { NewBugDialogSk } from '../new-bug-dialog-sk/new-bug-dialog-sk';
import { ExistingBugDialogSk } from '../existing-bug-dialog-sk/existing-bug-dialog-sk';
import { Anomaly } from '../json';
import { AnomalyData } from '../common/anomaly-data';
import { CountMetric, telemetry } from '../telemetry/telemetry';

import '../new-bug-dialog-sk/new-bug-dialog-sk';
import '../existing-bug-dialog-sk/existing-bug-dialog-sk';
import { ToastSk } from '../../../elements-sk/modules/toast-sk/toast-sk';

export class NudgeEntry {
  // Whether the nudge entry should be marked as selected in UI.
  selected: boolean = false;

  // AnomalyData contains the anomaly and its x, y coordinates.
  // If nudge is succesful, we modify its x and y with NudgeEntry.x
  // and NudgeEntry.y
  anomaly_data: AnomalyData | null = null;

  // start_revision to pass to backend to update DB value.
  start_revision: number = 0;

  // end_revision to pass to backend  to update DB value.
  end_revision: number = 0;

  // Number between -2 to 2 that indicates how many datapoints to nudge
  // an anomaly to.
  display_index: number = 0;

  // x value to update AnomalyData if user clicks on this entry.
  x: number = 0;

  // y value to update AnomalyData if user clicks on this entry.
  y: number = 0;
}

export class TriageMenuSk extends ElementSk {
  _trace_names: string[] = [];

  _anomalies: Anomaly[] = [];

  _nudgeList: NudgeEntry[] | null = null;

  _allowNudge: boolean = true;

  // New Bug Dialog.
  newBugDialog: NewBugDialogSk | null = null;

  // Existing Bug Dialog.
  existingBugDialog: ExistingBugDialogSk | null = null;

  private closeIgnoreToastButton: HTMLButtonElement | null = null;

  ignoreTriageToast: ToastSk | null = null;

  constructor() {
    super(TriageMenuSk.template);
  }

  private static template = (ele: TriageMenuSk) =>
    html`<div>
      <new-bug-dialog-sk></new-bug-dialog-sk>
      <existing-bug-dialog-sk></existing-bug-dialog-sk>
      ${ele._anomalies.length === 0 ? '' : ele.generateNudgeButtons()}
      <div class="buttons">
        <button id="new-bug" @click=${ele.fileBug}>New Bug</button>
        <button id="existing-bug" @click=${ele.openExistingBugDialog}>Existing Bug</button>
        <button id="ignore" ?hidden=${ele._anomalies.length === 0} @click=${ele.ignoreAnomaly}>
          Ignore
        </button>
      </div>
      <toast-sk id="ignore_toast" duration="8000">
        <div>
          <a> Anomaly has ignore with anomaly id: ${ele._anomalies.map((a) => a.id).join(', ')}</a>
        </div>
        <div class="close-bisect">
          <button id="hide-ignore-triage" class="action" type="button">Close</button>
        </div>
      </toast-sk>
    </div>`;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();

    this.existingBugDialog = this.querySelector('existing-bug-dialog-sk');
    this.newBugDialog = this.querySelector('new-bug-dialog-sk');
    this.closeIgnoreToastButton = this.querySelector('#hide-ignore-triage');
    this.ignoreTriageToast = this.querySelector('#ignore_toast');
    this.closeIgnoreToastButton!.addEventListener('click', () => this.ignoreTriageToast?.hide());

    this.addEventListener('click', (e) => {
      const existingBugButton = this.querySelector('#existing-bug');
      if (e.target === existingBugButton) {
        e.preventDefault();
        this.existingBugDialog!.fetch_associated_bugs();
      }
    });
  }

  fileBug() {
    telemetry.increaseCounter(CountMetric.TriageActionTaken, { action: 'file_bug' });
    this.newBugDialog!.fileNewBug();
  }

  openNewBugDialog() {
    this.newBugDialog!.open();
  }

  openExistingBugDialog() {
    telemetry.increaseCounter(CountMetric.TriageActionTaken, { action: 'associate_bug' });
    this.existingBugDialog!.open();
  }

  ignoreAnomaly() {
    telemetry.increaseCounter(CountMetric.TriageActionTaken, { action: 'ignore' });
    this.makeEditAnomalyRequest(this._anomalies, this._trace_names, 'IGNORE');
  }

  disableNudge() {
    this._allowNudge = false;
  }

  toggleButtons(enable: boolean) {
    const buttons = ['#new-bug', '#existing-bug', '#ignore'];
    buttons.forEach((btn) => {
      const b = this.querySelector(btn) as HTMLButtonElement;
      b.disabled = !enable;
    });
    this._render();
  }

  generateNudgeButtons(): TemplateResult {
    if (this._allowNudge === false) {
      return html``;
    }
    if (this._nudgeList === null) {
      return html``;
    }

    return html`
      <div class="buttons">
        <span id="tooltip-key">Nudge</span>
        ${this._nudgeList!.map(
          (entry) => html`
            <button
              value=${entry.display_index}
              class=${entry.selected ? 'selected' : ''}
              @click=${() => this.nudgeAnomaly(entry)}
              ?disabled=${entry.selected}>
              ${entry.display_index > 0 ? '+' + entry.display_index : entry.display_index}
            </button>
          `
        )}
      </div>
    `;
  }

  nudgeAnomaly(entry: NudgeEntry) {
    this.makeNudgeRequest(this._anomalies, this._trace_names, entry);
  }

  /**
   * Sends request to /_/triage/edit_anomalies API to edit anomaly data.
   *
   * If edit action is IGNORE, the anomaly will be ignored.
   * If edit action is RESET, the bug will be deassociated with the existing bug id.
   * If edit action is NUDGE, the anomaly will be nudged with a new time range.
   *
   * @param anomalies - The anomalies to modify.
   * @param traceNames - Trace IDs for modified anomalies. This tells the API to
   * invalidate these traces from the cache.
   * @param editAction - An action that corresponds to different behaviors.
   */
  makeEditAnomalyRequest(anomalies: Anomaly[], traceNames: string[], editAction: string): void {
    const keys: string[] = anomalies.map((a) => a.id);
    const body: any = {
      keys: keys,
      trace_names: traceNames,
      action: editAction,
    };

    fetch('/_/triage/edit_anomalies', {
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then(async () => {
        let bug_id: number | null = null;
        if (editAction === 'RESET') {
          bug_id = 0;
        } else if (editAction === 'IGNORE') {
          bug_id = -2;
        }
        if (bug_id !== null) {
          for (let i = 0; i < anomalies.length; i++) {
            anomalies[i].bug_id = bug_id;
          }
        }
        this.ignoreTriageToast?.show();
        this.dispatchEvent(
          new CustomEvent('anomaly-changed', {
            bubbles: true,
            composed: true,
            detail: {
              traceNames: traceNames,
              editAction: editAction,
              anomalies: anomalies,
            },
          })
        );
      })
      .catch(() => {
        errorMessage(
          'Edit anomalies request failed due to an internal server error. Please try again.'
        );
      });
  }

  /**
   * Sends request to /_/triage/edit_anomalies API to nudge anomalies.
   *
   * @param anomalies - Anomalies to edit, should usally be just 1.
   * @param traceNames - Trace IDs for modified anomalies. This tells the API to
   * invalidate these traces from the cache.
   * @param entry - NudgeEntry that is being edited. We want to update its x and y
   * values, as well as its selected value.
   */
  makeNudgeRequest(anomalies: Anomaly[], traceNames: string[], entry: NudgeEntry): void {
    const keys: string[] = anomalies.map((a) => a.id);
    const body: any = {
      keys: keys,
      trace_names: traceNames,
      action: 'NUDGE',
      start_revision: entry.start_revision,
      end_revision: entry.end_revision,
    };
    fetch('/_/triage/edit_anomalies', {
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then((_) => {
        for (let i = 0; i < anomalies.length; i++) {
          anomalies[i].start_revision = entry.start_revision;
          anomalies[i].end_revision = entry.end_revision;
        }

        this._nudgeList!.forEach((entry) => {
          entry.selected = false;
        });

        entry.selected = true;
        entry.anomaly_data!.x = entry.x;
        entry.anomaly_data!.y = entry.y;

        this.dispatchEvent(
          new CustomEvent('anomaly-changed', {
            bubbles: true,
            composed: true,
            detail: {
              traceNames: traceNames,
              anomalies: [entry.anomaly_data?.anomaly],
              displayIndex: entry.display_index,
            },
          })
        );
      })
      .catch((msg: any) => {
        errorMessage(msg);
      });
  }

  setAnomalies(anomalies: Anomaly[], traceNames: string[], nudgeList: NudgeEntry[] | null): void {
    this._anomalies = anomalies;
    this._trace_names = traceNames;
    this._nudgeList = nudgeList;
    this.newBugDialog!.setAnomalies(anomalies, traceNames);
    this.existingBugDialog!.setAnomalies(anomalies, traceNames);
    this._render();
  }
}

define('triage-menu-sk', TriageMenuSk);
