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
import { html, TemplateResult, LitElement } from 'lit';
import { customElement, property, query, state } from 'lit/decorators.js';
import { jsonOrThrow } from '../../../infra-sk/modules/jsonOrThrow';
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

@customElement('triage-menu-sk')
export class TriageMenuSk extends LitElement {
  @property({ attribute: false })
  traceNames: string[] = [];

  @property({ attribute: false })
  anomalies: Anomaly[] = [];

  @property({ attribute: false })
  nudgeList: NudgeEntry[] | null = null;

  @property({ type: Boolean })
  allowNudge: boolean = true;

  @state()
  private buttonsEnabled = true;

  // New Bug Dialog.
  @query('new-bug-dialog-sk')
  newBugDialog!: NewBugDialogSk;

  // Existing Bug Dialog.
  @query('existing-bug-dialog-sk')
  existingBugDialog!: ExistingBugDialogSk;

  @query('#ignore_toast')
  ignoreTriageToast!: ToastSk;

  createRenderRoot() {
    return this;
  }

  render() {
    return html`<div>
      <new-bug-dialog-sk
        .anomalies=${this.anomalies}
        .traceNames=${this.traceNames}></new-bug-dialog-sk>
      <existing-bug-dialog-sk
        .anomalies=${this.anomalies}
        .traceNames=${this.traceNames}></existing-bug-dialog-sk>
      ${this.anomalies.length === 0 ? '' : this.generateNudgeButtons()}
      <div class="buttons">
        <button id="new-bug" ?disabled=${!this.buttonsEnabled} @click=${this.fileBug}>
          New Bug
        </button>
        <button
          id="existing-bug"
          ?disabled=${!this.buttonsEnabled}
          @click=${this.openExistingBugDialog}>
          Existing Bug
        </button>
        <button
          id="ignore"
          ?hidden=${this.anomalies.length === 0}
          ?disabled=${!this.buttonsEnabled}
          @click=${this.ignoreAnomaly}>
          Ignore
        </button>
      </div>
      <toast-sk id="ignore_toast" duration="8000">
        <div>
          <a> Anomaly has ignore with anomaly id: ${this.anomalies.map((a) => a.id).join(', ')}</a>
        </div>
        <div class="close-bisect">
          <button
            id="hide-ignore-triage"
            class="action"
            type="button"
            @click=${() => this.ignoreTriageToast?.hide()}>
            Close
          </button>
        </div>
      </toast-sk>
    </div>`;
  }

  connectedCallback(): void {
    super.connectedCallback();
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
    this.existingBugDialog!.fetch_associated_bugs();
    this.existingBugDialog!.open();
  }

  ignoreAnomaly() {
    telemetry.increaseCounter(CountMetric.TriageActionTaken, { action: 'ignore' });
    this.makeEditAnomalyRequest(this.anomalies, this.traceNames, 'IGNORE');
  }

  disableNudge() {
    this.allowNudge = false;
  }

  toggleButtons(enable: boolean) {
    this.buttonsEnabled = enable;
  }

  generateNudgeButtons(): TemplateResult {
    if (this.allowNudge === false) {
      return html``;
    }
    if (this.nudgeList === null) {
      return html``;
    }

    return html`
      <div class="buttons">
        <span id="tooltip-key">Nudge</span>
        ${this.nudgeList!.map(
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
    this.makeNudgeRequest(this.anomalies, this.traceNames, entry);
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

        this.nudgeList!.forEach((entry) => {
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
    this.anomalies = anomalies;
    this.traceNames = traceNames;
    this.nudgeList = nudgeList;
  }
}
