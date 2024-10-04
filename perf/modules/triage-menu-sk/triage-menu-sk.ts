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
import { html } from 'lit/html.js';
import { define } from '../../../elements-sk/modules/define';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
import { ElementSk } from '../../../infra-sk/modules/ElementSk';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';
import { NewBugDialogSk } from '../new-bug-dialog-sk/new-bug-dialog-sk';
import { ExistingBugDialogSk } from '../existing-bug-dialog-sk/existing-bug-dialog-sk';
import { Anomaly } from '../json';

export class TriageMenuSk extends ElementSk {
  private _trace_names: string[] = [];

  private _anomalies: Anomaly[] = [];

  // New Bug Dialog.
  newBugDialog: NewBugDialogSk | null = null;

  // Existing Bug Dialog.
  existingBugDialog: ExistingBugDialogSk | null = null;

  constructor() {
    super(TriageMenuSk.template);
  }

  private static template = (ele: TriageMenuSk) =>
    html` <div>
      <new-bug-dialog-sk></new-bug-dialog-sk>
      <button id="new-bug" @click=${ele.openNewBugDialog}>New Bug</button>
      <existing-bug-dialog-sk></existing-bug-dialog-sk>
      <button id="existing-bug" @click=${ele.openExistingBugDialog}>Existing Bug</button>
      <button id="ignore" @click=${ele.ignoreAnomaly}>Ignore</button>
    </div>`;

  connectedCallback(): void {
    super.connectedCallback();
    this._render();

    this.existingBugDialog = this.querySelector('existing-bug-dialog-sk');
    this.newBugDialog = this.querySelector('new-bug-dialog-sk');
  }

  private openNewBugDialog() {
    this.newBugDialog!.open();
  }

  private openExistingBugDialog() {
    this.existingBugDialog!.open();
  }

  private ignoreAnomaly() {
    this.makeEditAnomalyRequest(this._anomalies, this._trace_names, 'IGNORE', null, null);
  }

  /**
   * Sends request to /_/triage/edit_anomalies API to edit anomaly data.
   *
   * If start_revision and end_revision are specified, it'll shift the given
   * anomalies to the specified revision range.
   *
   * If those are not specified, it does the following:
   * If bug_id is set to 0, it de-associates all bugs from the input anomalies.
   * If bug_id is set to -1, the anomaly is marked as invalid.
   * If bug_id is set to -2, the anomaly is marked as ignored.
   *
   * @param anomalies - The anomalies to modify.
   * @param traceNames - Trace IDs for modified anomalies. This tells the API to
   * invalidate these traces from the cache.
   * @param bug_id - Bug ID to set for all anomalies.
   * @param start_revision - start_revision to set for all anomalies.
   * @param end_revision - end_revision to set for all anomalies.
   */
  makeEditAnomalyRequest(
    anomalies: Anomaly[],
    traceNames: string[],
    editAction: string,
    start_revision: number | null,
    end_revision: number | null
  ): void {
    const keys: number[] = anomalies.map((a) => a.id);
    const body: any = { keys: keys, trace_names: traceNames, action: editAction };

    if (start_revision !== null) {
      body.start_revision = start_revision;
    }

    if (end_revision !== null) {
      body.end_revision = end_revision;
    }

    fetch('/_/triage/edit_anomalies', {
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
      },
    })
      .then(jsonOrThrow)
      .then((_) => {
        let bug_id = null;
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
        dispatchEvent(
          new CustomEvent('anomaly-changed', {
            bubbles: true,
          })
        );
      })
      .catch((msg: any) => {
        errorMessage(msg);
      });
  }

  setAnomalies(anomalies: Anomaly[], traceNames: string[]): void {
    this._anomalies = anomalies;
    this._trace_names = traceNames;
    this.newBugDialog!.setAnomalies(anomalies, traceNames);
    this.existingBugDialog!.setAnomalies(anomalies, traceNames);
    this._render();
  }
}

define('triage-menu-sk', TriageMenuSk);
