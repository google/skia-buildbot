/**
 * @module modules/triage-menu-sk
 * @description <h2><code>triage-menu-sk</code></h2>
 *
 * Triage Menu provides functionality to triage anomalies in bulk. These are the provided features:
 * - New Bug: Creaes a bug.
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
import { AnomalyData } from '../plot-simple-sk/plot-simple-sk';
import '../new-bug-dialog-sk/new-bug-dialog-sk';
import '../existing-bug-dialog-sk/existing-bug-dialog-sk';

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
  private _trace_names: string[] = [];

  private _anomalies: Anomaly[] = [];

  private _nudgeList: NudgeEntry[] | null = null;

  private _allowNudge: boolean = true;

  // New Bug Dialog.
  newBugDialog: NewBugDialogSk | null = null;

  // Existing Bug Dialog.
  existingBugDialog: ExistingBugDialogSk | null = null;

  constructor() {
    super(TriageMenuSk.template);
  }

  private static template = (ele: TriageMenuSk) =>
    html`<div>
      <new-bug-dialog-sk></new-bug-dialog-sk>
      <button id="new-bug" @click=${ele.openNewBugDialog}>New Bug</button>
      <existing-bug-dialog-sk></existing-bug-dialog-sk>
      <button id="existing-bug" @click=${ele.openExistingBugDialog}>Existing Bug</button>
      <button id="ignore" @click=${ele.ignoreAnomaly}>Ignore</button>
      ${ele.generateNudgeButtons()}
    </div>`;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();

    this.existingBugDialog = this.querySelector('existing-bug-dialog-sk');
    this.newBugDialog = this.querySelector('new-bug-dialog-sk');

    this.addEventListener('click', (e) => {
      const existingBugButton = this.querySelector('#existing-bug');
      if (e.target === existingBugButton) {
        e.preventDefault();
        this.existingBugDialog!.fetch_associated_bugs();
      }
    });
  }

  private openNewBugDialog() {
    this.newBugDialog!.open();
  }

  private openExistingBugDialog() {
    this.existingBugDialog!.open();
  }

  private ignoreAnomaly() {
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
  }

  generateNudgeButtons(): TemplateResult {
    if (this._allowNudge === false) {
      return html``;
    }
    if (this._nudgeList === null) {
      return html``;
    }

    return html`
      <div id="nudge-container">
        Nudge:
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

  private nudgeAnomaly(entry: NudgeEntry) {
    this.makeNudgeRequest(this._anomalies, this._trace_names, entry);
  }

  /**
   * Sends request to /_/triage/edit_anomalies API to edit anomaly data.
   *
   * If bug_id is set to 0, it de-associates all bugs from the input anomalies.
   * If bug_id is set to -1, the anomaly is marked as invalid.
   * If bug_id is set to -2, the anomaly is marked as ignored.
   *
   * @param anomalies - The anomalies to modify.
   * @param traceNames - Trace IDs for modified anomalies. This tells the API to
   * invalidate these traces from the cache.
   * @param bug_id - Bug ID to set for all anomalies.
   */
  makeEditAnomalyRequest(anomalies: Anomaly[], traceNames: string[], editAction: string): void {
    const keys: number[] = anomalies.map((a) => a.id);
    const body: any = { keys: keys, trace_names: traceNames, action: editAction };

    fetch('/_/triage/edit_anomalies', {
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then((_) => {
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
      .catch((msg: any) => {
        errorMessage(msg);
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
    const keys: number[] = anomalies.map((a) => a.id);
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
